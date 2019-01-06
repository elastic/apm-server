from apmserver import AccessTest

import requests


class Test(AccessTest):

    def test_with_token(self):
        """
        Test that access works with token
        """

        url = 'http://localhost:8200/intake/v2/events'
        events = self.get_event_payload()
        headers = {'content-type': 'application/x-ndjson'}

        def oauth(v):
            aheaders = {'Authorization': v}
            aheaders.update(headers)
            return aheaders

        r = requests.post(url, data=events, headers=headers)
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          data=events,
                          headers=oauth('Bearer 1234'))
        assert r.status_code == 202, r.status_code

        r = requests.post(url,
                          data=events,
                          headers=oauth('Bearer wrongtoken'))
        assert r.status_code == 401, r.status_code

        r = requests.post(url,
                          data=events,
                          headers=oauth('Wrongbearer 1234'))
        assert r.status_code == 401, r.status_code
