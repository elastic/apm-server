from apmserver import AccessTest

import requests


class Test(AccessTest):

    def test_with_token(self):
        """
        Test that access works with token
        """

        url = 'http://localhost:8200/v1/transactions'
        transactions = self.get_transaction_payload()

        def oauth(v): return {'Authorization': v}

        r = requests.post(url, json=transactions)
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          json=transactions,
                          headers=oauth('Bearer 1234'))
        assert r.status_code == 202, r.status_code

        r = requests.post(url,
                          json=transactions,
                          headers=oauth('Bearer wrongtoken'))
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          json=transactions,
                          headers=oauth('Wrongbearer 1234'))
        assert r.status_code == 401, r.status_code
