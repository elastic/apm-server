from apmserver import ElasticTest, ClientSideBaseTest, SmapCacheBaseTest
from beat.beat import INTEGRATION_TESTS
import os
import json
import requests
import unittest
import time


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

        self.check_backend_transaction(count=5)

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

        self.check_backend_error(count=4)


class SourcemappingIntegrationTest(ElasticTest, ClientSideBaseTest):

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_no_sourcemap_mapping_for_backend_error(self):
        path = 'http://localhost:8000/test/e2e/general-usecase/bundle.js.map'
        r = self.upload_sourcemap(file_name='bundle.js.map', bundle_filepath=path)
        assert r.status_code == 202, r.status_code
        self.wait_for_sourcemaps()

        self.load_docs_with_template(self.get_error_payload_path(),
                                     'http://localhost:8200/v1/errors',
                                     'error',
                                     1)
        self.assert_no_logged_warnings()
        self.check_backend_error()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_sourcemap_mappingES_for_error(self):
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
        self.check_error_smap(True, count=1)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_no_sourcemap_mapping_for_backend_transaction(self):
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
        self.check_backend_transaction()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_sourcemap_mappingES_for_transaction(self):
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
        self.check_transaction_smap(True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_no_sourcemap(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.errors_url,
                                     'error',
                                     1)
        self.check_error_smap(False, expected_err="No Sourcemap available for")

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
        self.check_error_smap(False, expected_err="No mapping for")

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
        self.check_error_smap(True, count=1)

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
        self.check_error_smap(True)


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
        self.check_error_smap(False, expected_err="No Sourcemap available for")
