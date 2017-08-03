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
    def test_load_docs_with_template(self):
        """
        This test starts the beat with a loaded template and sends data to elasticsearch.
        It verifies that all data makes it into ES means data is compatible with the template.
        """

        # TODO Needs cleanup when https://github.com/elastic/beats/pull/4769 merged
        base_name = "apmserver-tests"
        beat_version = "0.1.0"
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

        f = os.path.abspath(os.path.join(self.beat_path,
                                         'tests',
                                         'data',
                                         'valid',
                                         'transaction',
                                         'payload.json'))
        transactions = json.loads(open(f).read())
        url = 'http://localhost:8080/v1/transactions'
        r = requests.post(url, json=transactions)
        assert r.status_code == 201

        # make sure template is loaded
        self.wait_until(
            lambda: self.log_contains("Elasticsearch template with name 'apm-server-tests-0.1.0' loaded"))

        self.wait_until(lambda: es.indices.exists(index_name))
        es.indices.refresh(index=index_name)

        self.wait_until(lambda: es.count(index=index_name)['count'] == 9)

        res = es.count(index=index_name)
        assert 9 == res['count']

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
