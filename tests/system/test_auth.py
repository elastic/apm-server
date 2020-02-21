import base64
import json
import os
import requests

from apmserver import ServerBaseTest, ElasticTest
from apmserver import TimeoutError, integration_test
from test_apikey_cmd import APIKeyBase
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


class TestAccessWithSecretToken(ServerBaseTest):
    def config(self):
        cfg = super(TestAccessWithSecretToken, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg

    def test_backend_intake(self):
        """
        Test that access works with token
        """

        events = self.get_event_payload()

        r = requests.post(self.intake_url, data=events, headers=headers(""))
        assert r.status_code == 401, r.status_code

        r = requests.post(self.intake_url, data=events, headers=headers('Bearer 1234'))
        assert r.status_code == 202, r.status_code


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
        self.apikey = APIKeyBase(self.get_elasticsearch_url(user, password))

        # delete all existing api_keys with defined name of current user
        self.apikey.invalidate(self.apikey_name)
        self.apikey.wait_until_invalidated(name=self.apikey_name)
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

        def assert_intake(api_key, authorized):
            resp = requests.post(self.intake_url, data=self.get_event_payload(), headers=headers(api_key))
            if authorized:
                assert resp.status_code != 401, "token: {}, status_code: {}".format(api_key, resp.status_code)
            else:
                assert resp.status_code == 401, "token: {}, status_code: {}".format(api_key, resp.status_code)

        # fill cache up until one spot
        for i in range(4):
            assert_intake("ApiKey xyz{}".format(i), authorized=False)

        # allow for authorized api key
        assert_intake(key1, True)
        # hit cache size
        assert_intake(key2, False)
        # still allow already cached api key
        assert_intake(key1, True)


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
        assert resp.status_code == 401,  "token: {}, status_code: {}".format(key, resp.status_code)


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


@integration_test
class TestAccessWithAuthorization(APIKeyBaseTest):

    def setUp(self):
        super(TestAccessWithAuthorization, self).setUp()

        self.api_key_privileges_all_resource_any = self.create_api_key_header(self.privileges_all, self.resource_any)
        self.api_key_privileges_all_resource_backend = self.create_api_key_header(
            self.privileges_all, self.resource_backend)
        self.api_key_privilege_any_resource_any = self.create_api_key_header(self.privilege_any, self.resource_any)
        self.api_key_privilege_any_resource_backend = self.create_api_key_header(
            self.privilege_any, self.resource_backend)

        self.api_key_privilege_event = self.create_api_key_header([self.privilege_event], self.resource_any)
        self.api_key_privilege_config = self.create_api_key_header([self.privilege_agent_config], self.resource_any)
        self.api_key_privilege_sourcemap = self.create_api_key_header([self.privilege_sourcemap], self.resource_any)

        self.api_key_invalid_application = self.create_api_key_header(
            self.privileges_all, self.resource_any, application="foo")
        self.api_key_invalid_privilege = self.create_api_key_header(["foo"], self.resource_any)
        self.api_key_invalid_resource = self.create_api_key_header(self.privileges_all, "foo")

        self.authorized_keys = ["Bearer 1234",
                                self.api_key_privileges_all_resource_any, self.api_key_privileges_all_resource_backend,
                                self.api_key_privilege_any_resource_any, self.api_key_privilege_any_resource_backend]

        self.unauthorized_keys = ['', 'Bearer ', 'Bearer wrongtoken', 'Wrongbearer 1234',
                                  self.api_key_invalid_privilege, self.api_key_invalid_resource, "ApiKey nonexisting"]

    def config(self):
        cfg = super(TestAccessWithAuthorization, self).config()
        cfg.update({"secret_token": "1234", "api_key_enabled": True, "enable_rum": True,
                    "kibana_enabled": "true", "kibana_host": self.get_kibana_url()})
        return cfg

    def test_root(self):
        """
        Test authorization logic for root endpoint
        """
        url = self.root_url

        for token in self.unauthorized_keys:
            resp = requests.get(url, headers=headers(token))
            assert resp.status_code == 200, "token: {}, status_code: {}".format(token, resp.status_code)
            assert resp.text == '', "token: {}, response: {}".format(token, resp.content)

        keys_one_privilege = [self.api_key_privilege_config,
                              self.api_key_privilege_sourcemap, self.api_key_privilege_event]
        for token in self.authorized_keys+keys_one_privilege:
            resp = requests.get(url, headers=headers(token))
            assert resp.status_code == 200,  "token: {}, status_code: {}".format(token, resp.status_code)
            assert resp.content != '',  "token: {}, response: {}".format(token, resp.content)
            for token in ["build_date", "build_sha", "version"]:
                assert token in resp.json(), "token: {}, response: {}".format(token, resp.content)

    def test_backend_intake(self):
        """
        Test authorization logic for backend Intake endpoint
        """
        url = self.intake_url
        events = self.get_event_payload()

        for token in self.authorized_keys+[self.api_key_privilege_event]:
            resp = requests.post(url, data=events, headers=headers(token))
            assert resp.status_code == 202,  "token: {}, status_code: {}".format(token, resp.status_code)

        for token in self.unauthorized_keys+[self.api_key_privilege_config, self.api_key_privilege_sourcemap]:
            resp = requests.post(url, data=events, headers=headers(token))
            assert resp.status_code == 401,  "token: {}, status_code: {}".format(token, resp.status_code)

    def test_rum_intake(self):
        """
        Test authorization logic for RUM Intake endpoint.
        """
        url = self.rum_intake_url
        events = self.get_event_payload()

        # Endpoint is not secured, all keys are expected to be allowed.
        for token in self.authorized_keys + self.unauthorized_keys:
            resp = requests.post(url, data=events, headers=headers(token))
            assert resp.status_code != 401,  "token: {}, status_code: {}".format(token, resp.status_code)

    def test_agent_config(self):
        """
        Test authorization logic for backend Agent Configuration endpoint
        """
        url = self.agent_config_url

        for token in self.authorized_keys+[self.api_key_privilege_config]:
            resp = requests.get(url,
                                params={"service.name": "myservice"},
                                headers=headers(token, content_type="application/json"))
            assert resp.status_code == 200,  "token: {}, status_code: {}".format(token, resp.status_code)

        for token in self.unauthorized_keys+[self.api_key_privilege_event, self.api_key_privilege_sourcemap]:
            resp = requests.get(url, headers=headers(token, content_type="application/json"))
            assert resp.status_code == 401,  "token: {}, status_code: {}".format(token, resp.status_code)

    def test_rum_agent_config(self):
        """
        Test authorization logic for RUM Agent Configuration endpoint
        """
        url = self.rum_agent_config_url

        # Endpoint is not secured, all keys are expected to be allowed.
        for token in self.authorized_keys + self.unauthorized_keys:
            resp = requests.get(url, headers=headers(token, content_type="application/json"))
            assert resp.status_code != 401, "token: {}, status_code: {}".format(token, resp.status_code)

    def test_sourcemap(self):
        """
        Test authorization logic for Sourcemap upload endpoint
        """
        def upload(token):
            f = open(self._beat_path_join('testdata', 'sourcemap', 'bundle_no_mapping.js.map'))
            resp = requests.post(self.sourcemap_url,
                                 headers=headers(token, content_type=None),
                                 files={'sourcemap': f},
                                 data={'service_version': '1.0.1',
                                       'bundle_filepath': 'mapping.js.map',
                                       'service_name': 'apm-agent-js'
                                       })
            return resp

        for token in self.unauthorized_keys+[self.api_key_privilege_config, self.api_key_privilege_event]:
            resp = upload(token)
            assert resp.status_code == 401, "token: {}, status_code: {}".format(token, resp.status_code)

        for token in self.authorized_keys+[self.api_key_privilege_sourcemap]:
            resp = upload(token)
            assert resp.status_code == 202, "token: {}, status_code: {}".format(token, resp.status_code)
