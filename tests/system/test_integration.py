import os
import unittest

from apmserver import ElasticTest, ExpvarBaseTest
from apmserver import ClientSideBaseTest, SmapIndexBaseTest, SmapCacheBaseTest
from apmserver import SplitIndicesTest
from beat.beat import INTEGRATION_TESTS
import json


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
        It verifies that all data make it into ES, means data is compatible with the template
        and data are in expected format.
        """
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.transactions_url, 'transaction', 9)
        self.assert_no_logged_warnings()

        # compare existing ES documents for transactions with new ones
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "transaction"}}})
        assert rs['hits']['total'] == 4, "found {} documents".format(rs['count'])
        with open(self._beat_path_join(os.path.dirname(__file__), 'transaction.approved.json')) as f:
            approved = json.load(f)
        self.check_docs(approved, rs['hits']['hits'], 'transaction')

        # compare existing ES documents for spans with new ones
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "span"}}})
        assert rs['hits']['total'] == 5, "found {} documents".format(rs['count'])
        with open(self._beat_path_join(os.path.dirname(__file__), 'spans.approved.json')) as f:
            approved = json.load(f)
        self.check_docs(approved, rs['hits']['hits'], 'span')

        self.check_backend_transaction_sourcemap(count=5)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_load_docs_with_template_and_add_error(self):
        """
        This test starts the beat with a loaded template and sends error data to elasticsearch.
        It verifies that all data make it into ES means data is compatible with the template.
        """
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url, 'error', 4)
        self.assert_no_logged_warnings()

        # compare existing ES documents for errors with new ones
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})
        assert rs['hits']['total'] == 4, "found {} documents".format(rs['count'])
        with open(self._beat_path_join(os.path.dirname(__file__), 'error.approved.json')) as f:
            approved = json.load(f)
        self.check_docs(approved, rs['hits']['hits'], 'error')

        self.check_backend_error_sourcemap(count=4)

    def check_docs(self, approved, received, doc_type):
        for rec_entry in received:
            checked = False
            rec = rec_entry['_source']
            rec_id = rec[doc_type]['id']
            for appr_entry in approved:
                appr = appr_entry['_source']
                if rec_id == appr[doc_type]['id']:
                    checked = True
                    self.assert_docs(rec[doc_type], appr[doc_type])
                    self.assert_docs(rec['context'], appr['context'])
                    self.assert_docs(rec['@timestamp'], appr['@timestamp'])
                    self.assert_docs(rec['processor'], appr['processor'])
            assert checked == True, "New entry with id {}".format(rec_id)

    def assert_docs(self, received, approved):
        assert approved == received, "expected:\n{}\nreceived:\n{}".format(self.dump(approved), self.dump(received))

    def dump(self, data):
        return json.dumps(data, indent=4, separators=(',', ': '))


class FrontendEnabledIntegrationTest(ClientSideBaseTest):
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
    def test_enrich_backend_event(self):
        self.load_docs_with_template(self.get_transaction_payload_path(name="payload.json"),
                                     'http://localhost:8200/v1/transactions', 'transaction', 9)

        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "transaction"}}})

        assert "ip" in rs['hits']['hits'][0]["_source"]["context"]["system"], rs['hits']

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_enrich_frontend_event(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)

        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})

        hits = rs['hits']['hits']
        for hit in hits:
            assert "ip" in hit["_source"]["context"]["user"], rs['hits']
            assert "user-agent" in hit["_source"]["context"]["user"], rs['hits']

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
        assert l_frames == library_frames, "found {}, expected {}".format(
            l_frames, library_frames)

    def count_library_frames(self, doc, lf):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            if frame.has_key("library_frame"):
                k = "true" if frame["library_frame"] == True else "false"
                lf[k] += 1
            else:
                lf["empty"] += 1


class SplitIndicesIntegrationTest(SplitIndicesTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_split_docs_into_separate_indices(self):
        # load error and transaction document to ES
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     4,
                                     query_index="test-apm*")
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.transactions_url,
                                     'transaction',
                                     9,
                                     query_index="test-apm*")

        # check that every document is indexed once (incl.1 onboarding doc)
        assert 14 == self.es.count(index="test-apm*")['count']

        # check that documents are split into separate indices
        ct = self.es.count(
            index="test-apm-error-12-12-2017",
            body={"query": {"term": {"processor.event": "error"}}}
        )['count']
        assert 4 == ct

        ct = self.es.count(
            index="test-apm-transaction-12-12-2017",
            body={"query": {"term": {"processor.event": "transaction"}}}
        )['count']
        assert 4 == ct

        ct = self.es.count(
            index="test-apm-span-12-12-2017",
            body={"query": {"term": {"processor.event": "span"}}}
        )['count']
        assert 5 == ct


class SourcemappingIntegrationTest(ClientSideBaseTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_backend_error(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     'http://localhost:8200/v1/errors',
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_backend_error_sourcemap()

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
        assert self.log_contains(
            "Multiple sourcemaps found"), "the 3rd fetch should query ES and find that there are 2 sourcemaps with the same caching key"

        self.assert_no_logged_warnings(
            ["WARN.*Overriding sourcemap", "WARN.*Multiple sourcemaps"])

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_frontend_error(self):
        # use an uncleaned path to test that path is cleaned in upload
        path = 'http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
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
        self.check_frontend_error_sourcemap(
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
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_frontend_error_sourcemap(
            False, expected_err="No Sourcemap found for")

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
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
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
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
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


class SourcemappingIntegrationChangedConfigTest(SmapIndexBaseTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_frontend_error_changed_index(self):
        # use an uncleaned path to test that path is cleaned in upload
        path = 'http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(
            file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_frontend_error_sourcemap(True)


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
        self.check_frontend_error_sourcemap(
            False, expected_err="No Sourcemap available for")


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
