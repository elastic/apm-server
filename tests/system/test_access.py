from apmserver import AccessTest

import requests


class Test(AccessTest):

    def test_with_token(self):
        """
        Test that access works with token
        """

        url = 'http://localhost:8080/v1/transactions'
        transactions = self.get_payload()

        ctype = {'Content-Type': 'application/json'}.items()

        def oauth(v): return {'Authorization': v}.items()

        r = requests.post(url, data=transactions, headers=dict(ctype))
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          data=transactions,
                          headers=dict(ctype + oauth('Bearer 1234')))
        assert r.status_code == 202, r.status_code

        r = requests.post(url,
                          data=transactions,
                          headers=dict(ctype + oauth('Bearer wrongtoken')))
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          data=transactions,
                          headers=dict(ctype + oauth('Wrongbearer 1234')))
        assert r.status_code == 401, r.status_code
