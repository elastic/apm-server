from apmserver import BaseTest

import os
import requests
import json


class Test(BaseTest):

    def test_with_token(self):
        """
        Test that access works with token
        """
        self.render_config_template(
            path=os.path.abspath(self.working_dir) + "/log/*",
            secret_token="1234",
        )

        f = os.path.abspath(os.path.join(self.beat_path,
                                         'tests',
                                         'data',
                                         'valid',
                                         'transaction',
                                         'payload.json'))
        transactions = json.loads(open(f).read())

        apmserver_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("apm-server is running"))

        url = 'http://localhost:8080/v1/transactions'
        r = requests.post(url, json=transactions)
        assert r.status_code == 401

        r = requests.post(url, json=transactions, headers={'Authorization': 'Bearer 1234'})
        assert r.status_code == 201

        r = requests.post(url, json=transactions, headers={'Authorization': 'Bearer wrongtoken'})
        assert r.status_code == 401

        r = requests.post(url, json=transactions, headers={'Authorization': 'Wrongbearer 1234'})
        assert r.status_code == 401

        exit_code = apmserver_proc.kill_and_wait()
        assert exit_code == 0
