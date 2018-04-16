from collections import defaultdict
import gzip
import json
import requests
import threading
import time
import zlib

from nose.tools import raises
from requests.exceptions import SSLError

from apmserver import ServerBaseTest, SecureServerBaseTest, ClientSideBaseTest, CorsBaseTest


try:
    from StringIO import StringIO
except ImportError:
    from io import StringIO


class Test(ServerBaseTest):

    def test_ok(self):
        transactions = self.get_transaction_payload()
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 202, r.status_code

    def test_empty(self):
        transactions = {}
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 400, r.status_code

    def test_not_existent(self):
        transactions = {}
        invalid_url = 'http://localhost:8200/transactionX'
        r = requests.post(invalid_url, json=transactions)
        assert r.status_code == 404, r.status_code

    def test_method_not_allowed(self):
        r = requests.get(self.transactions_url)
        assert r.status_code == 405, r.status_code

    def test_bad_json(self):
        r = requests.post(self.transactions_url, json="not json")
        assert r.status_code == 400, r.status_code

    def test_validation_fail(self):
        transactions = self.get_transaction_payload()
        # month and day swapped
        transactions["transactions"][0]["timestamp"] = "2017-30-05T18:53:27.154Z"
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 400, r.status_code
        assert "Problem validating JSON document against schema" in r.content, r.content

    def test_validation_2_fail(self):
        transactions = self.get_transaction_payload()
        # timezone offsets not allowed
        transactions["transactions"][0]["timestamp"] = "2017-05-30T18:53:27.154+00:20"
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 400, r.status_code
        assert "Problem validating JSON document against schema" in r.content, r.content

    def test_frontend_default_disabled(self):
        transactions = self.get_transaction_payload()
        r = requests.post(
            'http://localhost:8200/v1/client-side/transactions', json=transactions)
        assert r.status_code == 403, r.status_code

    def test_healthcheck(self):
        healtcheck_url = 'http://localhost:8200/healthcheck'
        r = requests.get(healtcheck_url)
        assert r.status_code == 200, r.status_code

    def test_gzip(self):
        transactions = json.dumps(self.get_transaction_payload())
        try:
            out = StringIO()
        except:
            out = io.BytesIO()

        with gzip.GzipFile(fileobj=out, mode="w") as f:
            try:
                f.write(transactions)
            except:
                f.write(bytes(transactions, 'utf-8'))

        r = requests.post(self.transactions_url, data=out.getvalue(),
                          headers={'Content-Encoding': 'gzip', 'Content-Type': 'application/json'})
        assert r.status_code == 202, r.status_code

    def test_deflate(self):
        transactions = json.dumps(self.get_transaction_payload())
        try:
            compressed_data = zlib.compress(transactions)
        except:
            compressed_data = zlib.compress(bytes(transactions, 'utf-8'))

        r = requests.post(self.transactions_url, data=compressed_data,
                          headers={'Content-Encoding': 'deflate', 'Content-Type': 'application/json'})
        assert r.status_code == 202, r.status_code

    def test_gzip_error(self):
        data = self.get_transaction_payload()

        r = requests.post(self.transactions_url, json=data,
                          headers={'Content-Encoding': 'gzip', 'Content-Type': 'application/json'})
        assert r.status_code == 400, r.status_code

    def test_deflate_error(self):
        data = self.get_transaction_payload()

        r = requests.post(self.transactions_url, data=data,
                          headers={'Content-Encoding': 'deflate', 'Content-Type': 'application/json'})
        assert r.status_code == 400, r.status_code

    def test_expvar_default(self):
        """expvar should not be exposed by default"""
        r = requests.get(self.expvar_url)
        assert r.status_code == 404, r.status_code


class SecureTest(SecureServerBaseTest):

    def test_https_ok(self):
        transactions = self.get_transaction_payload()
        r = requests.post("https://localhost:8200/v1/transactions",
                          json=transactions, verify=False)
        assert r.status_code == 202, r.status_code

    @raises(SSLError)
    def test_https_verify(self):
        transactions = self.get_transaction_payload()
        requests.post("https://localhost:8200/v1/transactions",
                      json=transactions)


class ClientSideTest(ClientSideBaseTest):

    def test_ok(self):
        transactions = self.get_transaction_payload()
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 202, r.status_code

    def test_error_ok(self):
        errors = self.get_error_payload()
        r = requests.post(self.errors_url, json=errors)
        assert r.status_code == 202, r.status_code

    def test_sourcemap_upload(self):
        r = self.upload_sourcemap(file_name='bundle.js.map')
        assert r.status_code == 202, r.status_code

    def test_sourcemap_upload_fail(self):
        path = self._beat_path_join(
            'tests',
            'data',
            'valid',
            'sourcemap',
            'bundle.js.map')
        file = open(path)
        r = requests.post(self.sourcemap_url,
                          files={'sourcemap': file})
        assert r.status_code == 400, r.status_code


class CorsTest(CorsBaseTest):

    def test_ok(self):
        transactions = self.get_transaction_payload()
        r = requests.post(self.transactions_url, json=transactions, headers={
            'Origin': 'http://www.elastic.co'})
        assert r.headers['Access-Control-Allow-Origin'] == 'http://www.elastic.co', r.headers
        assert r.status_code == 202, r.status_code

    def test_bad_origin(self):
        # origin must include protocol and match exactly the allowed origin
        transactions = self.get_transaction_payload()
        r = requests.post(self.transactions_url, json=transactions, headers={
                          'Origin': 'www.elastic.co'})
        assert r.status_code == 403, r.status_code

    def test_no_origin(self):
        transactions = self.get_transaction_payload()
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 403, r.status_code

    def test_preflight(self):
        transactions = self.get_transaction_payload()
        r = requests.options(self.transactions_url,
                             json=transactions,
                             headers={'Origin': 'http://www.elastic.co',
                                      'Access-Control-Request-Method': 'POST',
                                      'Access-Control-Request-Headers': 'Content-Type, Content-Encoding'})
        assert r.status_code == 200, r.status_code
        assert r.headers['Access-Control-Allow-Origin'] == 'http://www.elastic.co', r.headers
        assert r.headers['Access-Control-Allow-Headers'] == 'Content-Type, Content-Encoding, Accept', r.headers
        assert r.headers['Access-Control-Allow-Methods'] == 'POST, OPTIONS', r.headers
        assert r.headers['Vary'] == 'Origin', r.headers
        assert r.headers['Content-Length'] == '0', r.headers
        assert r.headers['Access-Control-Max-Age'] == '3600', r.headers

    def test_preflight_bad_headers(self):
        transactions = self.get_transaction_payload()
        for h in [{'Access-Control-Request-Method': 'POST'}, {'Origin': 'www.elastic.co'}]:
            r = requests.options(self.transactions_url,
                                 json=transactions,
                                 headers=h)
            assert r.status_code == 200, r.status_code
            assert 'Access-Control-Allow-Origin' not in r.headers.keys(), r.headers
            assert r.headers['Access-Control-Allow-Headers'] == 'Content-Type, Content-Encoding, Accept', r.headers
            assert r.headers['Access-Control-Allow-Methods'] == 'POST, OPTIONS', r.headers


class RateLimitTest(ClientSideBaseTest):

    def test_rate_limit(self):

        transactions = self.get_transaction_payload()
        threads = []
        codes = defaultdict(int)

        def fire():
            r = requests.post(self.transactions_url, json=transactions)
            codes[r.status_code] += 1
            return r.status_code

        for _ in range(10):
            threads.append(threading.Thread(target=fire))

        for t in threads:
            t.start()

        for t in threads:
            t.join()

        assert set(codes.keys()) == set([202, 429]), codes
        assert codes[429] == 4, codes  # considering burst

        time.sleep(1)
        assert fire() == 202

    def test_rate_limit_multiple_ips(self):

        transactions = self.get_transaction_payload()
        threads = []
        codes = defaultdict(int)

        def fire(x):
            ip = '10.11.12.13' if x % 2 else '10.11.12.14'
            r = requests.post(self.transactions_url, json=transactions, headers={
                              'X-Forwarded-For': ip})
            codes[r.status_code] += 1
            return r.status_code

        for x in range(14):
            threads.append(threading.Thread(target=fire, args=(x,)))

        for t in threads:
            t.start()

        for t in threads:
            t.join()

        assert set(codes.keys()) == set([202, 429]), codes
        # considering burst: 1 "too many requests" per ip
        assert codes[429] == 2, codes

        time.sleep(1)
        assert fire(0) == 202
        assert fire(1) == 202
