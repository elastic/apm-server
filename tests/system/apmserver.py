import sys
import os
import json
import shutil
sys.path.append('../../_beats/libbeat/tests/system')
from beat.beat import TestCase
from elasticsearch import Elasticsearch
import requests
import time


class BaseTest(TestCase):

    @classmethod
    def setUpClass(cls):
        cls.beat_name = "apm-server"
        cls.build_path = "../../build/system-tests/"
        cls.beat_path = "../../"
        cls.index_name = "test-apm-12-12-2017"
        super(BaseTest, cls).setUpClass()

    def get_transaction_payload_path(self, name="payload.json"):
        return os.path.abspath(os.path.join(self.beat_path,
                                            'tests',
                                            'data',
                                            'valid',
                                            'transaction',
                                            name))

    def get_transaction_payload(self):
        path = self.get_transaction_payload_path()
        return json.loads(open(path).read())

    def get_error_payload_path(self, name="payload.json"):
        return os.path.abspath(os.path.join(self.beat_path,
                                            'tests',
                                            'data',
                                            'valid',
                                            'error',
                                            name))

    def get_error_payload(self):
        path = self.get_error_payload_path()
        return json.loads(open(path).read())


class ServerSetUpBaseTest(BaseTest):

    transactions_url = 'http://localhost:8200/v1/transactions'
    errors_url = 'http://localhost:8200/v1/errors'

    def config(self):
        return {"ssl_enabled": "false",
                "path": os.path.abspath(self.working_dir) + "/log/*"}

    def setUp(self):
        super(ServerSetUpBaseTest, self).setUp()
        shutil.copy(self.beat_path + "/fields.yml", self.working_dir)

        self.render_config_template(**self.config())
        self.apmserver_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("Starting apm-server"))

    def assert_no_logged_warnings(self, replace=None):
        """
        Assert that the log file contains no ERR or WARN lines.
        """
        log = self.get_log()
        log = log.replace("WARN EXPERIMENTAL", "")
        log = log.replace("WARN BETA", "")
        # Jenkins runs as a Windows service and when Jenkins executes theses
        # tests the Beat is confused since it thinks it is running as a service.
        log = log.replace(
            "ERR Error: The service process could not connect to the service controller.", "")
        if replace:
            for r in replace:
                log = log.replace(r, "")
        self.assertNotRegexpMatches(log, "ERR|WARN")


class ServerBaseTest(ServerSetUpBaseTest):
    def tearDown(self):
        super(ServerBaseTest, self).tearDown()
        self.apmserver_proc.check_kill_and_wait()


class SecureServerBaseTest(ServerSetUpBaseTest):

    @classmethod
    def setUpClass(cls):
        super(SecureServerBaseTest, cls).setUpClass()

        try:
            import urllib3
            urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        except ImportError:
            pass

        try:
            from requests.packages import urllib3
            urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        except ImportError:
            pass

    def config(self):
        cfg = super(SecureServerBaseTest, self).config()
        cfg.update({
            "ssl_enabled": "true",
            "ssl_cert": "config/certs/cert.pem",
            "ssl_key": "config/certs/key.pem",
        })
        return cfg

    def tearDown(self):
        super(SecureServerBaseTest, self).tearDown()
        self.apmserver_proc.kill_and_wait()


class AccessTest(ServerBaseTest):

    def config(self):
        cfg = super(AccessTest, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg


class ElasticTest(ServerBaseTest):

    def config(self):
        cfg = super(ElasticTest, self).config()
        cfg.update({
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
            "index_name": self.index_name
        })
        return cfg

    def setUp(self):
        self.es = Elasticsearch([self.get_elasticsearch_url()])

        # Cleanup index and template first
        self.es.indices.delete(index=self.index_name, ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists(self.index_name))

        self.es.indices.delete_template(name=self.index_name, ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists_template(self.index_name))

        super(ElasticTest, self).setUp()

    def get_elasticsearch_url(self):
        """
        Returns an elasticsearch.Elasticsearch url built from the
        env variables like the integration tests.
        """
        return "http://{host}:{port}".format(
            host=os.getenv("ES_HOST", "localhost"),
            port=os.getenv("ES_PORT", "9200"),
        )

    def load_docs_with_template(self, data_path, url, endpoint, expected_events_count):

        payload = json.loads(open(data_path).read())
        r = requests.post(url, json=payload)
        assert r.status_code == 202

        # make sure template is loaded
        self.wait_until(
            lambda: self.log_contains("Elasticsearch template with name \'{}\' loaded".format(self.index_name)),
            max_timeout=20)
        self.wait_until(lambda: self.es.indices.exists(self.index_name))
        # Quick wait to give documents some time to be sent to the index
        # This is not required but speeds up the tests
        time.sleep(0.1)
        self.es.indices.refresh(index=self.index_name)

        self.wait_until(
            lambda: (self.es.count(index=self.index_name, body={
                "query": {"term": {"processor.name": endpoint}}}
            )['count'] == expected_events_count)
        )

    def check_backend_error_sourcemap(self, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            if "exception" in err:
                self.check_for_no_smap(err["exception"])
            if "log" in err:
                self.check_for_no_smap(err["log"])

    def check_backend_transaction_sourcemap(self, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "span"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            self.check_for_no_smap(doc["_source"]["span"])

    def check_for_no_smap(self, doc):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            assert "sourcemap" not in frame


class ClientSideBaseTest(ServerBaseTest):

    transactions_url = 'http://localhost:8200/v1/client-side/transactions'
    errors_url = 'http://localhost:8200/v1/client-side/errors'
    sourcemap_url = 'http://localhost:8200/v1/client-side/sourcemaps'

    @classmethod
    def setUpClass(cls):
        super(ClientSideBaseTest, cls).setUpClass()
        cls.smap_index_pattern = "test-apm*"

    def config(self):
        cfg = super(ClientSideBaseTest, self).config()
        cfg.update({"enable_frontend": "true",
                    "smap_index_pattern": self.smap_index_pattern,
                    "smap_cache_expiration": "200"})
        return cfg

    def get_transaction_payload_path(self, name='frontend.json'):
        return super(ClientSideBaseTest, self).get_transaction_payload_path(name)

    def get_error_payload_path(self, name='frontend.json'):
        return super(ClientSideBaseTest, self).get_error_payload_path(name)

    def upload_sourcemap(self, file_name='bundle_no_mapping.js.map',
                         service_name='apm-agent-js',
                         service_version='1.0.1',
                         bundle_filepath='bundle_no_mapping.js.map',
                         expected_ct=1):
        path = os.path.abspath(os.path.join(self.beat_path,
                                            'tests',
                                            'data',
                                            'valid',
                                            'sourcemap',
                                            file_name))
        f = open(path)
        r = requests.post(self.sourcemap_url,
                          files={'sourcemap': f},
                          data={'service_version': service_version,
                                'bundle_filepath': bundle_filepath,
                                'service_name': service_name
                                })
        return r

    def wait_for_sourcemaps(self, expected_ct=1):
        idx = self.smap_index_pattern
        self.wait_until(
            lambda: (self.es.count(index=idx, body={
                "query": {"term": {"processor.name": 'sourcemap'}}}
            )['count'] == expected_ct)
        )

    def check_frontend_error_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            if "exception" in err:
                self.check_smap(err["exception"], updated, expected_err)
            if "log" in err:
                self.check_smap(err["log"], updated, expected_err)

    def check_frontend_transaction_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "span"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            span = doc["_source"]["span"]
            self.check_smap(span, updated, expected_err)

    def check_smap(self, doc, updated, err=None):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            smap = frame["sourcemap"]
            if err is None:
                assert 'error' not in smap
            else:
                assert err in smap["error"]
            assert smap["updated"] == updated


class CorsBaseTest(ClientSideBaseTest):

    def config(self):
        cfg = super(CorsBaseTest, self).config()
        cfg.update({"allow_origins": ["http://www.elastic.co"]})
        return cfg


class SmapCacheBaseTest(ClientSideBaseTest):
    def config(self):
        cfg = super(SmapCacheBaseTest, self).config()
        cfg.update({"smap_cache_expiration": "1"})
        return cfg
