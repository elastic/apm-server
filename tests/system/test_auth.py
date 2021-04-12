import base64
import json
import os
import requests

from apmserver import ServerBaseTest, ElasticTest
from apmserver import TimeoutError, integration_test
from test_apikey_cmd import APIKeyHelper
from helper import wait_until


def headers(auth=None, content_type='application/x-ndjson'):
    h = {'content-type': content_type}
    if auth is not None:
        auth_headers = {'Authorization': auth}
        auth_headers.update(h)
        return auth_headers
    return h


class TestAccessDefault(ServerBaseTest):
    """
    Unsecured endpoints
    """

    def test_full_access(self):
        """
        Test that authorized API Key is not accepted when API Key usage is disabled
        """
        events = self.get_event_payload()

        # access without token allowed
        resp = requests.post(self.intake_url, data=events, headers=headers())
        assert resp.status_code == 202, resp.status_code

        # access with any Bearer token allowed
        resp = requests.post(self.intake_url, data=events, headers=headers(auth="Bearer 1234"))
        assert resp.status_code == 202, resp.status_code

        # access with any API Key allowed
        resp = requests.post(self.intake_url, data=events, headers=headers(auth=""))
        assert resp.status_code == 202, resp.status_code


@integration_test
class APIKeyBaseTest(ElasticTest):
    def setUp(self):
        # application
        self.application = "apm"

        # apm privileges
        self.privilege_agent_config = "config_agent:read"
        self.privilege_event = "event:write"
        self.privilege_sourcemap = "sourcemap:write"
        self.privileges = {
            "agentConfig": self.privilege_agent_config,
            "event": self.privilege_event,
            "sourcemap": self.privilege_sourcemap
        }
        self.privileges_all = list(self.privileges.values())
        self.privilege_any = "*"

        # resources
        self.resource_any = ["*"]
        self.resource_backend = ["-"]

        user = os.getenv("ES_USER", "apm_server_user")
        password = os.getenv("ES_PASS", "changeme")
        self.apikey_name = "apm-systemtest"
        self.apikey = APIKeyHelper(self.get_elasticsearch_url(user, password))

        # delete all existing api_keys with defined name of current user
        self.apikey.invalidate(self.apikey_name)
        # delete all existing application privileges to ensure they can be created for current user
        for p in self.privileges.keys():
            url = "{}/{}/{}".format(self.apikey.privileges_url, self.application, p)
            requests.delete(url)
            wait_until(lambda: requests.get(url).status_code == 404)

        super(APIKeyBaseTest, self).setUp()

    def create_api_key_header(self, privileges, resources, application="apm"):
        return "ApiKey {}".format(self.create_apm_api_key(privileges, resources, application=application))

    def create_apm_api_key(self, privileges, resources, application="apm"):
        payload = json.dumps({
            "name": self.apikey_name,
            "role_descriptors": {
                self.apikey_name + "role_desc": {
                    "applications": [
                        {"application": application, "privileges": privileges, "resources": resources}]}}})
        resp = self.apikey.create(payload)
        enc = "utf-8"
        return str(base64.b64encode("{}:{}".format(resp["id"], resp["api_key"]).encode(enc)), enc)


@integration_test
class TestAPIKeyCache(APIKeyBaseTest):
    def config(self):
        cfg = super(TestAPIKeyCache, self).config()
        cfg.update({"api_key_enabled": True, "api_key_limit": 5})
        return cfg

    def test_cache_full(self):
        """
        Test that authorized API Key is not accepted when cache is full
        api_key.limit: number of unique API Keys per minute => cache size
        """
        key1 = self.create_api_key_header([self.privilege_event], self.resource_any)
        key2 = self.create_api_key_header([self.privilege_event], self.resource_any)

        def assert_intake(api_key, expected_status=202):
            resp = requests.post(self.intake_url, data=self.get_event_payload(), headers=headers(api_key))
            assert resp.status_code == expected_status, "token: {}, status_code: {}".format(api_key, resp.status_code)

        # fill cache up until one spot
        for i in range(4):
            assert_intake("ApiKey xyz{}".format(i), 401)

        # allow for authorized api key
        assert_intake(key1)
        # hit cache size
        assert_intake(key2, 503)
        # still allow already cached api key
        assert_intake(key1)


@integration_test
class TestAPIKeyWithInvalidESConfig(APIKeyBaseTest):
    def config(self):
        cfg = super(TestAPIKeyWithInvalidESConfig, self).config()
        cfg.update({"api_key_enabled": True, "api_key_es": "localhost:9999"})
        return cfg

    def test_backend_intake(self):
        """
        API Key cannot be verified when invalid Elasticsearch instance configured
        """
        name = "system_test_invalid_es"
        key = self.create_api_key_header([self.privilege_event], self.resource_any)
        resp = requests.post(self.intake_url, data=self.get_event_payload(), headers=headers(key))
        assert resp.status_code == 503,  "token: {}, status_code: {}".format(key, resp.status_code)


@integration_test
class TestAPIKeyWithESConfig(APIKeyBaseTest):
    def config(self):
        cfg = super(TestAPIKeyWithESConfig, self).config()
        cfg.update({"api_key_enabled": True, "api_key_es": self.get_elasticsearch_url()})
        return cfg

    def test_backend_intake(self):
        """
        Use dedicated Elasticsearch configuration for API Key validation
        """
        key = self.create_api_key_header([self.privilege_event], self.resource_any)
        resp = requests.post(self.intake_url, data=self.get_event_payload(), headers=headers(key))
        assert resp.status_code == 202,  "token: {}, status_code: {}".format(key, resp.status_code)
