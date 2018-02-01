import os
import unittest

from apmserver import ElasticTest, ExpvarBaseTest, ClientSideBaseTest, SmapCacheBaseTest
from beat.beat import INTEGRATION_TESTS


class Test(ElasticTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_onboarding_doc(self):
        """
        This test starts the beat and checks that the onboarding doc has been published to ES
        """
        self.wait_until(lambda: self.es.indices.exists(self.index_name))
        self.es.indices.refresh(index=self.index_name)

        self.wait_until(
            lambda: (self.es.count(index=self.index_name)['count'] == 1)
        )

        # Makes sure no error or warnings were logged
        self.assert_no_logged_warnings()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_load_docs_with_template_and_add_transaction(self):
        """
        This test starts the beat with a loaded template and sends transaction data to elasticsearch.
        It verifies that all data make it into ES means data is compatible with the template.
        """
        f = os.path.abspath(os.path.join(self.beat_path,
                                         'tests',
                                         'data',
                                         'valid',
                                         'transaction',
                                         'payload.json'))
        self.load_docs_with_template(f, self.transactions_url, 'transaction', 9)
        self.assert_no_logged_warnings()

        rs = self.es.count(index=self.index_name, body={
                           "query": {"term": {"processor.event": "transaction"}}})
        assert rs['count'] == 4, "found {} documents".format(rs['count'])

        self.check_backend_transaction_sourcemap(count=5)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_load_docs_with_template_and_add_error(self):
        """
        This test starts the beat with a loaded template and sends error data to elasticsearch.
        It verifies that all data make it into ES means data is compatible with the template.
        """
        f = os.path.abspath(os.path.join(self.beat_path,
                                         'tests',
                                         'data',
                                         'valid',
                                         'error',
                                         'payload.json'))
        self.load_docs_with_template(f, self.errors_url, 'error', 4)
        self.assert_no_logged_warnings()

        self.check_backend_error_sourcemap(count=4)


class FrontendEnabledIntegrationTest(ElasticTest, ClientSideBaseTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_error(self):
        self.load_docs_with_template(self.get_error_payload_path(name="payload.json"),
                                     'http://localhost:8200/v1/errors',
                                     'error',
                                     4)
        self.check_library_frames({"true": 1, "false": 1, "empty": 2}, "error")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_frontend_error(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_library_frames({"true": 5, "false": 1, "empty": 0}, "error")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_transaction(self):
        self.load_docs_with_template(self.get_transaction_payload_path(name="payload.json"),
                                     'http://localhost:8200/v1/transactions',
                                     'transaction',
                                     9)
        self.check_library_frames({"true": 1, "false": 0, "empty": 1}, "span")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_frontend_transaction(self):
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.transactions_url,
                                     'transaction',
                                     2)
        self.check_library_frames({"true": 1, "false": 1, "empty": 0}, "span")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_grouping_key_for_error(self):
        # upload the same error, once via frontend, once via backend endpoint
        # check they don't have the same grouping key, as the
        # `frontend.exclude_from_grouping` should only be applied to the frontend error.
        self.load_docs_with_template(self.get_error_payload_path(),
                                     'http://localhost:8200/v1/errors',
                                     'error',
                                     1)
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     2)
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})
        docs = rs['hits']['hits']
        grouping_key1 = docs[0]["_source"]["error"]["grouping_key"]
        grouping_key2 = docs[1]["_source"]["error"]["grouping_key"]
        assert grouping_key1 != grouping_key2

    def check_library_frames(self, library_frames, event):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": event}}})
        l_frames = {"true": 0, "false": 0, "empty": 0}
        for doc in rs['hits']['hits']:
            if "error" in doc["_source"]:
                err = doc["_source"]["error"]
                if "exception" in err:
                    self.count_library_frames(err["exception"], l_frames)
                if "log" in err:
                    self.count_library_frames(err["log"], l_frames)
            elif "span" in doc["_source"]:
                span = doc["_source"]["span"]
                self.count_library_frames(span, l_frames)
        assert l_frames == library_frames, "found {}, expected {}".format(l_frames, library_frames)

    def count_library_frames(self, doc, lf):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            if frame.has_key("library_frame"):
                k = "true" if frame["library_frame"] == True else "false"
                lf[k] += 1
            else:
                lf["empty"] += 1


class SourcemappingIntegrationTest(ElasticTest, ClientSideBaseTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_error(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     'http://localhost:8200/v1/errors',
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_backend_error_sourcemap()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_frontend_error(self):
        # use an uncleaned path to test that path is cleaned in upload
        path = 'http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_frontend_error_sourcemap(True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_transaction(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map',
                                  bundle_filepath=path,
                                  service_version='1.0.0')
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     'http://localhost:8200/v1/transactions',
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_backend_transaction_sourcemap()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_frontend_transaction(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map',
                                  bundle_filepath=path,
                                  service_version='1.0.0')
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.transactions_url,
                                     'transaction',
                                     2)
        self.assert_no_logged_warnings()
        self.check_frontend_transaction_sourcemap(True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_no_sourcemap(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_frontend_error_sourcemap(False, expected_err="No Sourcemap available for")

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
        r = self.upload_sourcemap(file_name='bundle_no_mapping.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_frontend_error_sourcemap(False, expected_err="No Sourcemap found for")

        # remove existing document
        self.es.delete_by_query(index=self.index_name,
                                body={"query": {"term": {"processor.name": 'error'}}})
        self.wait_until(
            lambda: (self.es.count(index=self.index_name, body={
                "query": {"term": {"processor.name": 'error'}}}
            )['count'] == 0)
        )

        # upload second sourcemap file with same key,
        # that actually leads to proper matchings
        # this also tests that the cache gets invalidated,
        # as otherwise the former sourcemap would be taken from the cache.
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps(expected_ct=2)
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_frontend_error_sourcemap(True, count=1)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_sourcemap_mapping_cache_usage(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        # insert document, which also leads to caching the sourcemap
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()

        # delete sourcemap from ES
        # fetching from ES would lead to an error afterwards
        self.es.indices.delete(index=self.index_name, ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists(self.index_name))

        # insert document,
        # fetching sourcemap without errors, so it must be fetched from cache
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_frontend_error_sourcemap(True)


class SourcemappingCacheIntegrationTest(ElasticTest, SmapCacheBaseTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_sourcemap_cache_expiration(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        # insert document, which also leads to caching the sourcemap
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()

        # delete sourcemap from ES
        # fetching from ES would lead to an error afterwards
        self.es.indices.delete(index=self.index_name, ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists(self.index_name))

        # after cache expiration no sourcemap should be found any more
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_frontend_error_sourcemap(False, expected_err="No Sourcemap available for")


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
