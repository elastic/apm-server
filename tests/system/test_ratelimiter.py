from apmserver import BaseTest
import json
import os
import requests
from nose.plugins.skip import Skip, SkipTest
import multiprocessing as mp
import time
import shutil


class Test(BaseTest):

    def test_ratelimiter(self):
        """
        Test rate limiting feature.
        """

        # This test is currently skipped as for testing the rate limiter requests would have to be made in parallel
        #raise SkipTest

        self.render_config_template(
            path=os.path.abspath(self.working_dir) + "/log/*",
            ssl_enabled=False,
            ratelimit_size=1,
            ratelimit_burst=1,
        )

        shutil.copy(self.beat_path + "/fields.yml", self.working_dir)

        proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("apm-server start running"))
        self.wait_until(lambda: self.log_contains("Rate limiter enabled"))

        f = os.path.abspath(os.path.join(self.beat_path,
                                         'tests',
                                         'data', 'valid', 'transaction',
                                         'payload.json'))
        transactions = json.loads(open(f).read())
        url = 'http://localhost:8200/v1/transactions'

        def do_request(url, transactions, q):
            try:
                r = requests.post(url, json=transactions)
                q.put(r.status_code)
            except:
                pass

        q = mp.Queue()
        processes = [mp.Process(target=do_request, args=[url, transactions, q]) for x in range(1, 10)]

        for p in processes:
            p.start()

        for p in processes:
            p.join()

        self.wait_until(lambda: self.log_contains("Too many requests"))

        codes = {}
        while not q.empty():
            code = q.get()

            if not code in codes:
                codes[code] = 0
            codes[code] = codes[code] + 1

        # Prints which codes appeared, useful if tests fail for debugging
        print codes

        # There should be at least 2 events which went through
        assert codes[202] > 1

        # At least 1 event should have been rejected
        assert codes[429] >= 1

        exit_code = proc.kill_and_wait()
        assert exit_code == 0
