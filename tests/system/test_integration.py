from apmserver import BaseTest
from beat.beat import INTEGRATION_TESTS
import os
import json
import requests
import unittest
import shutil
from elasticsearch import Elasticsearch


class Test(BaseTest):

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
        self.load_docs_with_template(f, 'transactions', 9)

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
        self.load_docs_with_template(f, 'errors', 4)

    def load_docs_with_template(self, data_path, endpoint, expected_events_count):
        # TODO Needs cleanup when https://github.com/elastic/beats/pull/4769 merged
        base_name = "apm-server-tests"
        beat_version = "0.1.1"
        index_name = base_name + "-" + beat_version + "-1"

        self.render_config_template(
            path=os.path.abspath(self.working_dir) + "/log/*",
            elasticsearch_host=self.get_elasticsearch_url(),
            file_enabled="false",
            index_name=index_name,
            template_base_name=base_name,
        )

        es = Elasticsearch([self.get_elasticsearch_url()])

        # Cleanup index and template first
        try:
            es.indices.delete(index=index_name)
        except:
            pass
        self.wait_until(lambda: not es.indices.exists(index_name))

        try:
            es.indices.delete_template(name=base_name + "-" + beat_version)
        except:
            pass

        shutil.copy(self.beat_path + "/fields.yml", self.working_dir)

        # Start beat
        self.apmserver_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("apm-server is running"))

        payload = json.loads(open(data_path).read())
        url = 'http://localhost:8080/v1/' + endpoint
        r = requests.post(url, json=payload)
        assert r.status_code == 202

        # make sure template is loaded
        self.wait_until(
            lambda: self.log_contains("Elasticsearch template with name 'apm-server-tests-0.1.1' loaded"))

        self.wait_until(lambda: es.indices.exists(index_name))
        es.indices.refresh(index=index_name)

        self.wait_until(lambda: es.count(index=index_name)['count'] == expected_events_count)

        res = es.count(index=index_name)
        assert expected_events_count == res['count']
        self. apmserver_proc.check_kill_and_wait()

        # Makes sure no error or warnings were logged
        self.assert_no_logged_warnings()

    def get_elasticsearch_url(self):
        """
        Returns an elasticsearch.Elasticsearch instance built from the
        env variables like the integration tests.
        """
        return "http://{host}:{port}".format(
            host=os.getenv("ES_HOST", "localhost"),
            port=os.getenv("ES_PORT", "9200"),
        )

    def assert_no_logged_warnings(self, replace=None):
        """
        Assert that the log file contains no ERR or WARN lines.
        """
        log = self.get_log()
        log = log.replace("WARN EXPERIMENTAL", "")
        log = log.replace("WARN BETA", "")
        # Jenkins runs as a Windows service and when Jenkins executes theses
        # tests the Beat is confused since it thinks it is running as a service.
        log = log.replace("ERR Error: The service process could not connect to the service controller.", "")
        if replace:
            for r in replace:
                log = log.replace(r, "")
        self.assertNotRegexpMatches(log, "ERR|WARN")
