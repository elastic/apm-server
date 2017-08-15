import sys
import os
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


class ServerBaseTest(BaseTest):

    def setUp(self):
        super(ServerBaseTest, self).setUp()
        self.render_config_template(
            path=os.path.abspath(self.working_dir) + "/log/*"
        )
        self.apmserver_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("apm-server is running"))

    def tearDown(self):
        super(ServerBaseTest, self).tearDown()
        self.apmserver_proc.kill_and_wait()
