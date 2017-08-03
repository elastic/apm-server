import json

from apmserver import ServerBaseTest
import requests
import os
import zlib
import gzip
try:
    from StringIO import StringIO
except ImportError:
    import io


class Test(ServerBaseTest):
    transactions_url = 'http://localhost:8080/v1/transactions'

    def test_ok(self):
        transactions = self.get_payload()
        r = requests.post(self.transactions_url, data=transactions,
                          headers={'Content-Type': 'application/json'})
        assert r.status_code == 201

    def test_empty(self):
        transactions = {}
        r = requests.post(self.transactions_url, json=transactions)
        assert r.status_code == 400

    def test_not_existent(self):
        transactions = {}
        invalid_url = 'http://localhost:8080/transactionX'
        r = requests.post(invalid_url, json=transactions)
        assert r.status_code == 404

    def test_method_not_allowed(self):
        r = requests.get(self.transactions_url)
        assert r.status_code == 405

    def test_bad_json(self):
        r = requests.post(self.transactions_url, json="not json")
        assert r.status_code == 400

    def test_healthcheck(self):
        healtcheck_url = 'http://localhost:8080/healthcheck'
        r = requests.get(healtcheck_url)
        assert r.status_code == 200

    def test_gzip(self):
        transactions = self.get_payload()

        out = ""
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
        assert r.status_code == 201

    def test_deflat(self):
        transactions = self.get_payload()
        compressed_data = None
        try:
            compressed_data = zlib.compress(transactions)
        except:
            compressed_data = zlib.compress(bytes(transactions, 'utf-8'))

        r = requests.post(self.transactions_url, data=compressed_data,
                          headers={'Content-Encoding': 'deflate', 'Content-Type': 'application/json'})
        assert r.status_code == 201

    def test_gzip_error(self):
        data = self.get_payload()

        r = requests.post(self.transactions_url, data=data,
                          headers={'Content-Encoding': 'gzip', 'Content-Type': 'application/json'})
        assert r.status_code == 400

    def test_deflate_error(self):
        data = self.get_payload()

        r = requests.post(self.transactions_url, data=data,
                          headers={'Content-Encoding': 'deflate', 'Content-Type': 'application/json'})
        assert r.status_code == 400

    def get_payload(self):
        path = os.path.abspath(os.path.join(self.beat_path,
                                            'tests',
                                            'data',
                                            'valid',
                                            'transaction',
                                            'payload.json'))
        transactions = json.loads(open(path).read())
        return json.dumps(transactions)
