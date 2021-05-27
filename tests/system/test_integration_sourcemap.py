import time
from urllib.parse import urlparse, urlunparse
import requests
import json

from apmserver import integration_test
from apmserver import ClientSideElasticTest
from test_auth import APIKeyHelper
from helper import wait_until
from es_helper import index_smap, index_metric, index_transaction, index_error, index_span, index_onboarding, index_name


class BaseSourcemapTest(ClientSideElasticTest):
    def upload_sourcemap(self,
                         file_name='bundle.js.map',
                         bundle_filepath='http://localhost:8000/test/e2e/general-usecase/bundle.js.map',
                         service_name='apm-agent-js',
                         service_version='1.0.1',
                         expected_ct=1,
                         status_code=202):
        path = self._beat_path_join('testdata', 'sourcemap', file_name)
        with open(path) as f:
            r = requests.post(self.sourcemap_url,
                              files={'sourcemap': f},
                              data={'service_version': service_version,
                                    'bundle_filepath': bundle_filepath,
                                    'service_name': service_name})
            assert r.status_code == status_code, r.status_code
        if status_code < 400:
            self.wait_for_events('sourcemap', expected_ct, index=index_smap)

    def split_url(self, cfg):
        url = urlparse(cfg["elasticsearch_host"])
        url_parts = list(url)
        url_parts[1] = url.netloc.split('@')[-1]
        return {"host": urlunparse(url_parts), "username": url.username, "password": url.password}


@integration_test
class SourcemappingIntegrationTest(BaseSourcemapTest):
    def test_backend_error(self):
        # ensure source mapping is not applied to backend events
        # load event for which a sourcemap would be applied when sent to rum endpoint,
        # and send against backend endpoint.

        self.upload_sourcemap()
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.backend_intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_backend_error_sourcemap(index_error)

    def test_duplicated_sourcemap_warning(self):
        self.upload_sourcemap()
        self.upload_sourcemap(expected_ct=2)
        assert self.log_contains(
            "Overriding sourcemap"), "A log should be written when a sourcemap is overwritten"
        self.upload_sourcemap(expected_ct=3)
        assert self.log_contains("2 sourcemaps found"), \
            "the 3rd fetch should query ES and find that there are 2 sourcemaps with the same caching key"
        self.assert_no_logged_warnings(
            ["WARN.*Overriding sourcemap", "WARN.*2 sourcemaps found"])

    def test_backend_span(self):
        # ensure source mapping is not applied to backend events
        # load event for which a sourcemap would be applied when sent to rum endpoint,
        # and send against backend endpoint.
        self.upload_sourcemap(service_version='1.0.0')
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.backend_intake_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_backend_span_sourcemap()

    def test_rum_transaction(self):
        self.upload_sourcemap(service_version='1.0.0')
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.intake_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_rum_transaction_sourcemap(True)

    def test_rum_transaction_different_subdomain(self):
        self.upload_sourcemap(service_version='1.0.0')
        self.load_docs_with_template(self.get_payload_path('transactions_spans_rum_2.ndjson'),
                                     self.intake_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_rum_transaction_sourcemap(True)

    def test_no_sourcemap(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(
            False, expected_err="No Sourcemap available")

    def test_no_matching_sourcemap(self):
        self.upload_sourcemap(file_name='bundle_no_mapping.js.map', bundle_filepath='bundle_no_mapping.js.map')
        self.assert_no_logged_warnings()
        self.test_no_sourcemap()

    def test_fetch_latest_of_multiple_sourcemaps(self):
        # upload sourcemap file that finds no matchings
        self.upload_sourcemap(file_name='bundle_no_mapping.js.map')
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(
            False, expected_err="No Sourcemap found for")

        # remove existing document
        self.es.delete_by_query(index=index_error, body={"query": {"term": {"processor.name": 'error'}}})
        wait_until(lambda: (self.es.count(index=index_error)['count'] == 0))

        # upload second sourcemap file with same key,
        # that actually leads to proper matchings
        # this also tests that the cache gets invalidated,
        # as otherwise the former sourcemap would be taken from the cache.
        self.upload_sourcemap(expected_ct=2)
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(True, count=1)

    def test_sourcemap_mapping_cache_usage(self):
        self.upload_sourcemap()
        # insert document, which also leads to caching the sourcemap
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()

        # delete sourcemap from ES
        # fetching from ES would lead to an error afterwards
        self.es.indices.delete(index=index_smap, ignore=[400, 404])
        self.es.indices.delete(index="{}-000001".format(index_error), ignore=[400, 404])

        # insert document,
        # fetching sourcemap without errors, so it must be fetched from cache
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_rum_error_sourcemap(True)


@integration_test
class SourcemappingCacheIntegrationTest(BaseSourcemapTest):
    config_overrides = {"smap_cache_expiration": "1"}

    def test_sourcemap_cache_expiration(self):
        self.upload_sourcemap()

        # insert document, which also leads to caching the sourcemap
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()

        # delete sourcemap and error event from ES
        self.es.indices.delete(index=self.ilm_index(index_error))
        # fetching from ES will lead to an error afterwards
        self.es.indices.delete(index=index_smap, ignore=[400, 404])
        wait_until(lambda: not self.es.indices.exists(index_smap))
        # ensure smap is not in cache any more
        time.sleep(1)

        # after cache expiration no sourcemap should be found any more
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(False, expected_err="No Sourcemap available")


@integration_test
class SourcemappingDisabledIntegrationTest(BaseSourcemapTest):
    config_overrides = {
        "rum_sourcemapping_disabled": True,
    }

    def test_rum_transaction(self):
        self.upload_sourcemap(service_version='1.0.0', status_code=403)
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.intake_url,
                                     'transaction',
                                     2)
        rs = self.es.search(index=index_span, params={"rest_total_hits_as_int": "true"})
        assert rs['hits']['total'] == 1, "found {} documents, expected {}".format(
            rs['hits']['total'], 1)
        frames_checked = 0
        for doc in rs['hits']['hits']:
            span = doc["_source"]["span"]
            for frame in span["stacktrace"]:
                frames_checked += 1
                assert "sourcemap" not in frame, frame
        assert frames_checked > 0, "no frames checked"


@integration_test
class SourcemapInvalidESConfig(BaseSourcemapTest):
    def config(self):
        cfg = super(SourcemapInvalidESConfig, self).config()
        url = self.split_url(cfg)
        cfg.update({
            "smap_es_host": url["host"],
            "smap_es_username": url["username"],
            "smap_es_password": "xxxx",
        })
        return cfg

    def test_unauthorized(self):
        # successful - uses output.elasticsearch.* configuration
        self.upload_sourcemap()
        # unauthorized - uses apm-server.rum.sourcemapping.elasticsearch configuration
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        assert self.log_contains("unable to authenticate user")


@integration_test
class SourcemapESConfigUser(BaseSourcemapTest):
    def config(self):
        cfg = super(SourcemapESConfigUser, self).config()
        url = self.split_url(cfg)
        cfg.update({
            "smap_es_host": url["host"],
            "smap_es_username": url["username"],
            "smap_es_password": url["password"],
        })
        return cfg

    def test_sourcemap_applied(self):
        # uses output.elasticsearch.* configuration
        self.upload_sourcemap()
        # uses apm-server.rum.sourcemapping.elasticsearch configuration
        self.load_docs_with_template(self.get_error_payload_path(), self.intake_url, 'error', 1)
        self.assert_no_logged_warnings()
        self.check_rum_error_sourcemap(True)


@integration_test
class SourcemapESConfigAPIKey(BaseSourcemapTest):
    def config(self):
        cfg = super(SourcemapESConfigAPIKey, self).config()

        # create API Key that is valid for fetching source maps
        apikey = APIKeyHelper(self.get_elasticsearch_url())
        payload = json.dumps({
            'name': 'test_sourcemap_apikey',
            'role_descriptors': {
                'test_sourcemap_apikey': {
                    'index': [
                        {
                            'names': ['apm-*'],
                            'privileges': ['read']
                        }
                    ]
                }
            }
        })
        resp = apikey.create(payload)
        cfg.update({
            "smap_es_host": self.split_url(cfg)["host"],
            "smap_es_apikey": "{}:{}".format(resp["id"], resp["api_key"]),
        })
        return cfg

    def test_sourcemap_applied(self):
        # uses output.elasticsearch.* configuration
        self.upload_sourcemap()
        # uses apm-server.rum.sourcemapping.elasticsearch configuration
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_rum_error_sourcemap(True)
