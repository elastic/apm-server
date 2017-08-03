
from apmserver import ServerBaseTest
import os
import json
import requests
import time


class Test(ServerBaseTest):

    def test_generate_transaction_trace(self):
        """
        Generates example output documents for trace and transactions
        """
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

        self.wait_until(lambda: self.output_count(lambda x: x >= 2))

        output = self.read_output_json()

        for event in output:
            if 'context' in event.keys():
                del(event["context"])
            self.assert_fields_are_documented(event)

        for event in output:
            if 'context' in event.keys():
                del(event["context"])
            self.assert_fields_are_documented(event)

    def test_generate_error(self):
        """
        Generates example output documents for exceptions including stacktrace
        """
        f = os.path.abspath(os.path.join(self.beat_path,
                                         'tests', 'data', 'valid', 'error',
                                         'payload.json'))
        errors = json.loads(open(f).read())

        url = 'http://localhost:8080/v1/errors'
        r = requests.post(url, json=errors)
        assert r.status_code == 201

        self.wait_until(lambda: self.output_count(lambda x: x >= 1))

        output = self.read_output_json()

        for event in output:
            if 'context' in event.keys():
                del(event["context"])
            self.assert_fields_are_documented(event)
