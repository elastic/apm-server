from apmserver import BaseTest

import os


class Test(BaseTest):

    def test_base(self):
        """
        Basic test with exiting apmserver normally
        """
        self.render_config_template(
            path=os.path.abspath(self.working_dir) + "/log/*"
        )

        apmserver_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("apm-server is running"))
        exit_code = apmserver_proc.kill_and_wait()
        assert exit_code == 0
