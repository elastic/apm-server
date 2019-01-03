from apmserver import AccessTest

import requests


class Test(AccessTest):

    def test_with_token(self):
        """
        Test that access works with token
        """

        url = 'http://localhost:8200/intake/v2/events'
        transactions = self.get_transaction_payload()
        headers = {'content-type': 'application/x-ndjson'}

        def oauth(v):
            aheaders = {'Authorization': v}
            aheaders.update(headers)
            return aheaders

        r = requests.post(url, data=transactions, headers=headers)
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          data=transactions,
                          headers=oauth('Bearer 1234'))
        assert r.status_code == 202, r.status_code

        r = requests.post(url,
                          data=transactions,
                          headers=oauth('Bearer wrongtoken'))
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          data=transactions,
                          headers=oauth('Wrongbearer 1234'))
        assert r.status_code == 401, r.status_code
