import sys
import os
import json
import shutil
sys.path.append('../../_beats/libbeat/tests/system')
from beat.beat import TestCase
from elasticsearch import Elasticsearch

# TODO: rework system tests in go


class BaseTest(TestCase):

    @classmethod
    def setUpClass(cls):
        cls.beat_name = "apm-server"
        cls.build_path = "../../build/system-tests/"
        cls.beat_path = "../../"
        super(BaseTest, cls).setUpClass()

    def get_transaction_payload(self):
        path = os.path.abspath(os.path.join(self.beat_path,
                                            'tests',
                                            'data',
                                            'valid',
                                            'transaction',
                                            'payload.json'))
        transactions = json.loads(open(path).read())
        return json.dumps(transactions)


class ServerBaseTest(BaseTest):

    def config(self):
        return {
            "ssl_enabled": "false",
            "path": os.path.abspath(self.working_dir) + "/log/*"
        }

    def setUp(self):
        super(ServerBaseTest, self).setUp()
        self.render_config_template(**self.config())
        self.apmserver_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("apm-server is running"))

    def tearDown(self):
        super(ServerBaseTest, self).tearDown()
        self.apmserver_proc.check_kill_and_wait()


class SecureServerBaseTest(ServerBaseTest):

    def config(self):
        cfg = super(SecureServerBaseTest, self).config()
        cfg.update({
            "ssl_enabled": "true",
            "ssl_cert": "config/certs/cert.pem",
            "ssl_key": "config/certs/key.pem",
        })
        return cfg


class AccessTest(ServerBaseTest):

    def config(self):
        cfg = super(AccessTest, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg


class ElasticTest(ServerBaseTest):

    @classmethod
    def setUpClass(cls):
        super(ElasticTest, cls).setUpClass()
        cls.index_name = "apm-server-tests"

    def config(self):
        cfg = super(ElasticTest, self).config()
        cfg.update({"elasticsearch_host": self.get_elasticsearch_url(),
                    "file_enabled": "false",
                    "index_name": self.index_name})
        return cfg

    def setUp(self):
        super(ElasticTest, self).setUp()

        self.es = Elasticsearch([self.get_elasticsearch_url()])

        # Cleanup index and template first
        try:
            self.es.indices.delete(index=self.index_name)
        except:
            pass
        self.wait_until(lambda: not self.es.indices.exists(self.index_name))

        try:
            self.es.indices.delete_template(
                name=self.index_name)
        except:
            pass

        shutil.copy(self.beat_path + "/fields.yml", self.working_dir)

    def get_elasticsearch_url(self):
        """
        Returns an elasticsearch.Elasticsearch instance built from the
        env variables like the integration tests.
        """
        return "http://{host}:{port}".format(
            host=os.getenv("ES_HOST", "localhost"),
            port=os.getenv("ES_PORT", "9200"),
        )
