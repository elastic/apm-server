import os
import json
import unittest
import time

from apmserver import ElasticTest, ExpvarBaseTest
from apmserver import ClientSideElasticTest, SmapCacheBaseTest
from apmserver import OverrideIndicesTest, OverrideIndicesFailureTest
from beat.beat import INTEGRATION_TESTS
from sets import Set


class Test(ElasticTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_onboarding_doc(self):
        """
        This test starts the beat and checks that the onboarding doc has been published to ES
        """
        self.wait_until(lambda: self.es.indices.exists(self.index_onboarding))
        self.es.indices.refresh(index=self.index_onboarding)

        self.wait_until(
            lambda: (self.es.count(index=self.index_onboarding)['count'] == 1)
        )

        # Makes sure no error or warnings were logged
        self.assert_no_logged_warnings()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_template(self):
        """
        This test starts the beat and checks that the template has been loaded to ES
        """
        self.wait_until(lambda: self.es.indices.exists(self.index_onboarding))
        self.es.indices.refresh(index=self.index_onboarding)
        templates = self.es.indices.get_template(self.index_name)
        assert len(templates) == 1
        t = templates[self.index_name]
        total_fields_limit = t['settings']['index']['mapping']['total_fields']['limit']
        assert total_fields_limit == "2000", total_fields_limit

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_tags_type(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()
        mappings = self.es.indices.get_field_mapping(index=self.index_transaction, fields="context.tags.*")
        for name, metric in mappings[self.index_transaction]["mappings"].items():
            fullname = metric["full_name"]
            for mapping in metric["mapping"].values():
                mtype = mapping["type"]
                if fullname.startswith("context.tags.bool"):
                    assert mtype == "boolean", name + " mapped as " + mtype + ", not boolean"
                elif fullname.startswith("context.tags.number"):
                    assert mtype == "scaled_float", name + " mapped as " + mtype + ", not scaled_float"
                else:
                    assert mtype == "keyword", name + " mapped as " + mtype + ", not keyword"

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_mark_type(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()
        mappings = self.es.indices.get_field_mapping(index=self.index_transaction, fields="transaction.marks.*")
        for name, metric in mappings[self.index_transaction]["mappings"].items():
            for mapping in metric["mapping"].values():
                mtype = mapping["type"]
                assert mtype == "scaled_float", name + " mapped as " + mtype + ", not scaled_float"

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_load_docs_with_template_and_add_transaction(self):
        """
        This test starts the beat with a loaded template and sends transaction data to elasticsearch.
        It verifies that all data make it into ES, means data is compatible with the template
        and data are in expected format.
        """
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()

        # compare existing ES documents for transactions with new ones
        rs = self.es.search(index=self.index_transaction)
        assert rs['hits']['total']['value'] == 4, "found {} documents".format(rs['count'])
        with open(self._beat_path_join(os.path.dirname(__file__), 'transaction.approved.json')) as f:
            approved = json.load(f)
        self.check_docs(approved, rs['hits']['hits'], 'transaction')

        # compare existing ES documents for spans with new ones
        rs = self.es.search(index=self.index_span)
        assert rs['hits']['total']['value'] == 5, "found {} documents".format(rs['count'])
        with open(self._beat_path_join(os.path.dirname(__file__), 'spans.approved.json')) as f:
            approved = json.load(f)
        self.check_docs(approved, rs['hits']['hits'], 'span')

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_load_docs_with_template_and_add_error(self):
        """
        This test starts the beat with a loaded template and sends error data to elasticsearch.
        It verifies that all data make it into ES means data is compatible with the template.
        """
        self.load_docs_with_template(self.get_error_payload_path(), self.intake_url, 'error', 4)
        self.assert_no_logged_warnings()

        # compare existing ES documents for errors with new ones
        rs = self.es.search(index=self.index_error)
        assert rs['hits']['total']['value'] == 4, "found {} documents".format(rs['count'])
        with open(self._beat_path_join(os.path.dirname(__file__), 'error.approved.json')) as f:
            approved = json.load(f)
        self.check_docs(approved, rs['hits']['hits'], 'error')

        self.check_backend_error_sourcemap(self.index_error, count=4)

    def check_docs(self, approved, received, doc_type):
        assert len(received) == len(approved)
        for rec_entry in received:
            checked = False
            rec = rec_entry['_source']
            rec_id = rec[doc_type]['id']
            for appr_entry in approved:
                appr = appr_entry['_source']
                if rec_id == appr[doc_type]['id']:
                    checked = True
                    for k, v in rec.items():
                        if k == "observer":
                            expected = Set(["hostname", "version", "id", "ephemeral_id", "type", "version_major"])
                            rec_keys = Set(v.keys())
                            assert len(expected.symmetric_difference(rec_keys)) == 0
                            assert v["version"].startswith(str(v["version_major"]) + ".")
                            continue

                        self.assert_docs(v, appr[k])
            assert checked, "New entry with id {}".format(rec_id)

    def assert_docs(self, received, approved):
        assert approved == received, "expected:\n{}\nreceived:\n{}".format(self.dump(approved), self.dump(received))

    def dump(self, data):
        return json.dumps(data, indent=4, separators=(',', ': '))


class EnrichEventIntegrationTest(ClientSideElasticTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_error(self):
        # for backend events librar_frame information should not be changed,
        # as no regex pattern is defined.
        self.load_docs_with_template(self.get_backend_error_payload_path(),
                                     self.backend_intake_url,
                                     'error',
                                     4)
        self.check_library_frames({"true": 1, "false": 1, "empty": 2}, self.index_error)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_rum_error(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_library_frames({"true": 5, "false": 1, "empty": 0}, self.index_rum_error)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_transaction(self):
        # for backend events librar_frame information should not be changed,
        # as no regex pattern is defined.
        self.load_docs_with_template(self.get_backend_transaction_payload_path(),
                                     self.backend_intake_url,
                                     'transaction',
                                     9)
        self.check_library_frames({"true": 1, "false": 0, "empty": 1}, self.index_span)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_rum_transaction(self):
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.intake_url,
                                     'transaction',
                                     2)
        self.check_library_frames({"true": 1, "false": 1, "empty": 0}, self.index_rum_span)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_enrich_backend_event(self):
        self.load_docs_with_template(self.get_backend_transaction_payload_path(),
                                     self.backend_intake_url, 'transaction', 9)

        rs = self.es.search(index=self.index_transaction)

        assert "ip" in rs['hits']['hits'][0]["_source"]["host"], rs['hits']

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_enrich_rum_event(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)

        rs = self.es.search(index=self.index_rum_error)

        hits = rs['hits']['hits']
        for hit in hits:
            assert "user_agent" in hit["_source"], rs['hits']
            assert "original" in hit["_source"]["user_agent"], rs['hits']
            assert "ip" in hit["_source"]["client"], rs['hits']

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_grouping_key_for_error(self):
        # upload the same error, once via rum, once via backend endpoint
        # check they don't have the same grouping key, as the
        # `rum.exclude_from_grouping` should only be applied to the rum error.
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.backend_intake_url,
                                     'error',
                                     2)

        rs = self.es.search(index=self.index_rum_error)
        docs = rs['hits']['hits']
        grouping_key1 = docs[0]["_source"]["error"]["grouping_key"]
        grouping_key2 = docs[1]["_source"]["error"]["grouping_key"]
        assert grouping_key1 != grouping_key2

    def check_library_frames(self, library_frames, index_name):
        rs = self.es.search(index=index_name)
        l_frames = {"true": 0, "false": 0, "empty": 0}
        for doc in rs['hits']['hits']:
            if "error" in doc["_source"]:
                err = doc["_source"]["error"]
                for exception in err.get("exception", []):
                    self.count_library_frames(exception, l_frames)
                if "log" in err:
                    self.count_library_frames(err["log"], l_frames)
            elif "span" in doc["_source"]:
                span = doc["_source"]["span"]
                self.count_library_frames(span, l_frames)
        assert l_frames == library_frames, "found {}, expected {}".format(
            l_frames, library_frames)

    @staticmethod
    def count_library_frames(doc, lf):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            if frame.has_key("library_frame"):
                k = "true" if frame["library_frame"] else "false"
                lf[k] += 1
            else:
                lf["empty"] += 1


class OverrideIndicesIntegrationTest(OverrideIndicesTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_override_indices_config(self):
        # load error and transaction document to ES
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     4,
                                     query_index=self.index_name)
        self.load_docs_with_template(self.get_payload_path("transactions_spans_rum.ndjson"),
                                     self.intake_url,
                                     'transaction',
                                     2,
                                     query_index=self.index_name)

        # check that every document is indexed once in the expected index (incl.1 onboarding doc)
        assert 7 == self.es.count(index=self.index_name)['count']


class OverrideIndicesFailureIntegrationTest(OverrideIndicesFailureTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_template_setup_error(self):
        loaded_msg = "Exiting: setup.template.name and setup.template.pattern have to be set"
        self.wait_until(lambda: self.log_contains(loaded_msg),
                        max_timeout=5)


class SourcemappingIntegrationTest(ClientSideElasticTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_error(self):
        # ensure source mapping is not applied to backend events
        # load event for which a sourcemap would be applied when sent to rum endpoint,
        # and send against backend endpoint.

        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.backend_intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_backend_error_sourcemap(self.index_rum_error)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_duplicated_sourcemap_warning(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'

        self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        self.wait_for_sourcemaps()

        self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        self.wait_for_sourcemaps(2)
        assert self.log_contains(
            "Overriding sourcemap"), "A log should be written when a sourcemap is overwritten"

        self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        self.wait_for_sourcemaps(3)
        assert self.log_contains("Multiple sourcemaps found"), \
            "the 3rd fetch should query ES and find that there are 2 sourcemaps with the same caching key"

        self.assert_no_logged_warnings(
            ["WARN.*Overriding sourcemap", "WARN.*Multiple sourcemaps"])

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_rum_error(self):
        # use an uncleaned path to test that path is cleaned in upload
        path = 'http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_rum_error_sourcemap(True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_span(self):
        # ensure source mapping is not applied to backend events
        # load event for which a sourcemap would be applied when sent to rum endpoint,
        # and send against backend endpoint.

        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map',
                                  bundle_filepath=path,
                                  service_version='1.0.0')
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.backend_intake_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_backend_span_sourcemap()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_rum_transaction(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map',
                                  bundle_filepath=path,
                                  service_version='1.0.0')
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.intake_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_rum_transaction_sourcemap(True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_rum_transaction_different_subdomain(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map',
                                  bundle_filepath=path,
                                  service_version='1.0.0')
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_payload_path('transactions_spans_rum_2.ndjson'),
                                     self.intake_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_rum_transaction_sourcemap(True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_no_sourcemap(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(
            False, expected_err="No Sourcemap available for")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_no_matching_sourcemap(self):
        r = self.upload_sourcemap('bundle_no_mapping.js.map')
        self.assert_no_logged_warnings()
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()
        self.test_no_sourcemap()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_fetch_latest_of_multiple_sourcemaps(self):
        # upload sourcemap file that finds no matchings
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle_no_mapping.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(
            False, expected_err="No Sourcemap found for")

        # remove existing document
        self.es.delete_by_query(index=self.index_rum_error, body={"query": {"term": {"processor.name": 'error'}}})
        self.wait_until(lambda: (self.es.count(index=self.index_rum_error)['count'] == 0))

        # upload second sourcemap file with same key,
        # that actually leads to proper matchings
        # this also tests that the cache gets invalidated,
        # as otherwise the former sourcemap would be taken from the cache.
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps(expected_ct=2)
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(True, count=1)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_sourcemap_mapping_cache_usage(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        # insert document, which also leads to caching the sourcemap
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()

        # delete sourcemap from ES
        # fetching from ES would lead to an error afterwards
        self.es.indices.delete(index=self.index_smap, ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists(self.index_smap))

        # insert document,
        # fetching sourcemap without errors, so it must be fetched from cache
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_rum_error_sourcemap(True)


class SourcemappingIntegrationChangedConfigTest(ClientSideElasticTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_rum_error_changed_index(self):
        # use an uncleaned path to test that path is cleaned in upload
        path = 'http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_rum_error_sourcemap(True)


class SourcemappingCacheIntegrationTest(SmapCacheBaseTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_sourcemap_cache_expiration(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        # insert document, which also leads to caching the sourcemap
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()

        # delete sourcemap and error event from ES
        self.es.indices.delete(index=self.index_rum_error)
        # fetching from ES will lead to an error afterwards
        self.es.indices.delete(index=self.index_smap, ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists(self.index_smap))
        # ensure smap is not in cache any more
        time.sleep(1)

        # after cache expiration no sourcemap should be found any more
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_rum_error_sourcemap(False, expected_err="No Sourcemap available for")


class ExpvarDisabledIntegrationTest(ExpvarBaseTest):
    config_overrides = {"expvar_enabled": "false"}

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_expvar_exists(self):
        """expvar disabled, should 404"""
        r = self.get_debug_vars()
        assert r.status_code == 404, r.status_code


class ExpvarEnabledIntegrationTest(ExpvarBaseTest):
    config_overrides = {"expvar_enabled": "true"}

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_expvar_exists(self):
        """expvar enabled, should 200"""
        r = self.get_debug_vars()
        assert r.status_code == 200, r.status_code


class ExpvarCustomUrlIntegrationTest(ExpvarBaseTest):
    config_overrides = {"expvar_enabled": "true", "expvar_url": "/foo"}
    expvar_url = ExpvarBaseTest.expvar_url.replace("/debug/vars", "/foo")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_expvar_exists(self):
        """expvar enabled, should 200"""
        r = self.get_debug_vars()
        assert r.status_code == 200, r.status_code


class MetricsIntegrationTest(ElasticTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_metric_doc(self):
        self.load_docs_with_template(self.get_metricset_payload_payload_path(), self.intake_url, 'metric', 2)
        mappings = self.es.indices.get_field_mapping(
            index=self.index_metric, fields="system.process.cpu.total.norm.pct")
        expected_type = "scaled_float"
        doc = mappings[self.index_metric]["mappings"]
        actual_type = doc["system.process.cpu.total.norm.pct"]["mapping"]["pct"]["type"]
        assert expected_type == actual_type, "want: {}, got: {}".format(expected_type, actual_type)


class ExperimentalBaseTest(ElasticTest):
    def check_experimental_key_indexed(self, experimental):
        loaded_msg = "No pipeline callback registered"
        self.wait_until(lambda: self.log_contains(loaded_msg), max_timeout=5)
        self.load_docs_with_template(self.get_payload_path("experimental.ndjson"),
                                     self.intake_url, 'transaction', 2)

        self.assert_no_logged_warnings()

        for idx in [self.index_transaction, self.index_span, self.index_error]:
            # ensure documents exist
            rs = self.es.search(index=idx)
            assert rs['hits']['total']['value'] == 1

            # check whether or not top level key `experimental` has been indexed
            rs = self.es.search(index=idx, body={"query": {"exists": {"field": 'experimental'}}})
            ct = 1 if experimental else 0
            assert rs['hits']['total']['value'] == ct


class ProductionModeTest(ExperimentalBaseTest):
    config_overrides = {"mode": "production"}

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_experimental_key_indexed(self):
        self.check_experimental_key_indexed(False)


class ExperimentalModeTest(ExperimentalBaseTest):
    config_overrides = {"mode": "experimental"}

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_experimental_key_indexed(self):
        self.check_experimental_key_indexed(True)
