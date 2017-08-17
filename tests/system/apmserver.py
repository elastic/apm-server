import sys
import os
import json
sys.path.append('../../_beats/libbeat/tests/system')
from beat.beat import TestCase


# TODO: rework system tests in go
class BaseTest(TestCase):

    @classmethod
    def setUpClass(cls):
        cls.beat_name = "apm-server"
        cls.build_path = "../../build/system-tests/"
        cls.beat_path = "../../"
        super(BaseTest, cls).setUpClass()

    def get_payload(self):
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
            "host": "localhost:8080",
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
        exit_code = self.apmserver_proc.kill_and_wait()
        assert exit_code == 0


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
