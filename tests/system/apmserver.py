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

    # TODO: rewrite system test in go
    def assert_fields_are_documented(self, evt):
        """
        Assert that all keys present in evt are documented in fields.yml.
        This reads from the global fields.yml,
        means `make collect` has to be run before the check.
        """
        expected_fields, dict_fields = self.load_fields()
        flat = self.flatten_object(evt, dict_fields)

        for key in flat.keys():
            # Allow any keys for "tags" subgroup.
            if key not in expected_fields and not key.startswith("transaction.tags."):
                raise Exception("Key '{}' found in event is not documented!"
                                .format(key))


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
