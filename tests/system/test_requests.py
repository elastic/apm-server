from nose.tools import raises

from apmserver import ServerBaseTest, SecureServerBaseTest
from requests.exceptions import SSLError
import requests
import json
import zlib
import gzip
try:
    from StringIO import StringIO
except ImportError:
    import io


class Test(ServerBaseTest):
    transactions_url = 'http://localhost:8200/v1/transactions'

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
