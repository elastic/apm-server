from apmserver import ServerBaseTest
import requests


class Test(ServerBaseTest):

    def test_health(self):
        r = requests.get("http://localhost:8080/healthcheck")
        assert r.status_code == 200, r.status_code
