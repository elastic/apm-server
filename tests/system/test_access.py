from apmserver import ServerBaseTest, ElasticTest
from apmserver import TimeoutError, integration_test
from helper import wait_until

import base64
import json
import os
import requests
import shutil
import ssl
import subprocess

from nose.tools import raises
from requests.exceptions import SSLError, ChunkedEncodingError
from requests.packages.urllib3.exceptions import SubjectAltNameWarning
requests.packages.urllib3.disable_warnings(SubjectAltNameWarning)

INTEGRATION_TESTS = os.environ.get('INTEGRATION_TESTS', False)


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
class BaseAPIKey(ElasticTest):

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
        self.privileges_all = self.privileges.values()
        self.privilege_any = "*"

        # resources
        self.resource_any = ["*"]
        self.resource_backend = ["-"]

        self.api_key_name = "apm-systemtest"
        content_type = 'application/json'

        # api_key related urls for configured user (default: apm_server_user)
        user = os.getenv("ES_USER", "apm_server_user")
        password = os.getenv("ES_PASS", "changeme")
        self.es_url_apm_server_user = self.get_elasticsearch_url(user, password)
        self.api_key_url = "{}/_security/api_key".format(self.es_url_apm_server_user)
        self.privileges_url = "{}/_security/privilege".format(self.es_url_apm_server_user)

        # clean setup:
        # delete all existing api_keys with defined name of current user
        requests.delete(self.api_key_url,
                        data=json.dumps({'name': self.api_key_name}),
                        headers=headers(content_type='application/json'))
        wait_until(lambda: self.api_keys_invalidated(), name="delete former api keys")
        # delete all existing application privileges to ensure they can be created for current user
        for name in self.privileges.keys():
            url = "{}/{}/{}".format(self.privileges_url, self.application, name)
            requests.delete(url)
            wait_until(lambda: requests.get(url).status_code == 404)

        super(BaseAPIKey, self).setUp()

    def fetch_api_keys(self):
        resp = requests.get("{}?name={}".format(self.api_key_url, self.api_key_name))
        assert resp.status_code == 200
        assert "api_keys" in resp.json(), resp.json()
        return resp.json()["api_keys"]

    def api_keys_invalidated(self):
        for entry in self.fetch_api_keys():
            if not entry["invalidated"]:
                return False
        return True

    def api_key_exists(self, id):
        resp = requests.get("{}?id={}".format(self.api_key_url, id))
        assert resp.status_code == 200, resp.status_code
        return len(resp.json()["api_keys"]) == 1

    def create_api_key(self, privileges, resources, application="apm"):
        payload = json.dumps({
            "name": self.api_key_name,
            "role_descriptors": {
                self.api_key_name + "role_desc": {
                    "applications": [
                        {"application": application, "privileges": privileges, "resources": resources}]}}})
        resp = requests.post(self.api_key_url,
                             data=payload,
                             headers=headers(content_type='application/json'))
        assert resp.status_code == 200, resp.status_code
        id = resp.json()["id"]
        wait_until(lambda: self.api_key_exists(id), name="create api key")
        return base64.b64encode("{}:{}".format(id, resp.json()["api_key"]))

    def create_api_key_header(self, privileges, resources, application="apm"):
        return "ApiKey {}".format(self.create_api_key(privileges, resources, application=application))


@integration_test
class TestAPIKeyCache(BaseAPIKey):
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
class TestAPIKeyWithInvalidESConfig(BaseAPIKey):
    def config(self):
        cfg = super(TestAPIKeyWithInvalidESConfig, self).config()
        cfg.update({"api_key_enabled": True, "api_key_es": "localhost:9999"})
        return cfg

    def test_backend_intake(self):
        """
        API Key cannot be verified when invalid Elasticsearch instance configured
        """
        key = self.create_api_key_header([self.privilege_event], self.resource_any)
        resp = requests.post(self.intake_url, data=self.get_event_payload(), headers=headers(key))
        assert resp.status_code == 401,  "token: {}, status_code: {}".format(key, resp.status_code)


@integration_test
class TestAPIKeyWithESConfig(BaseAPIKey):
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
class TestAccessWithAuthorization(BaseAPIKey):

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
            assert resp.content == '', "token: {}, response: {}".format(token, resp.content)

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


@integration_test
class TestSecureServerBaseTest(ServerBaseTest):
    @classmethod
    def setUpClass(cls):
        # According to https://docs.python.org/2/library/unittest.html#setupclass-and-teardownclass setUp and tearDown
        # should be skipped when class is skipped, which is apparently not true.
        # This is a hack to avoid running the setup while it should be skipped
        if not INTEGRATION_TESTS:
            return
        cls.config_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "config"))
        cls.cert_path = os.path.join(cls.config_path, "certs")
        shutil.rmtree(cls.cert_path, ignore_errors=True)
        cls.create_certs_cmd = os.path.join(cls.config_path, "create_certs.sh")
        with open(os.devnull, 'wb') as dev_null:
            subprocess.call([cls.create_certs_cmd, cls.config_path, cls.cert_path,
                             "foobar"], stdout=dev_null, stderr=dev_null)
        super(TestSecureServerBaseTest, cls).setUpClass()

    @classmethod
    def tearDownClass(cls):
        if not INTEGRATION_TESTS:
            return
        super(TestSecureServerBaseTest, cls).tearDownClass()
        shutil.rmtree(cls.cert_path)

    def setUp(self):
        self.config_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "config"))
        self.cert_path = os.path.join(self.config_path, "certs")
        self.ca_cert = os.path.join(self.cert_path, "ca.crt.pem")
        self.simple_cert = os.path.join(self.cert_path, "simple.crt.pem")
        self.simple_key = os.path.join(self.cert_path, "simple.key.pem")
        self.client_cert = os.path.join(self.cert_path, "client.crt.pem")
        self.client_key = os.path.join(self.cert_path, "client.key.pem")
        self.server_cert = os.path.join(self.cert_path, "server.crt.pem")
        self.server_key = os.path.join(self.cert_path, "server.key.pem")
        self.password = "foobar"
        self.host = "localhost"
        self.port = 8200
        super(TestSecureServerBaseTest, self).setUp()

    def stop_proc(self):
        self.apmserver_proc.kill_and_wait()

    def ssl_overrides(self):
        return {}

    def config(self):
        cfg = super(TestSecureServerBaseTest, self).config()
        overrides = {
            "ssl_enabled": "true",
            "ssl_certificate": self.server_cert,
            "ssl_key": self.server_key,
            "ssl_key_passphrase": self.password
        }
        cfg.update(overrides)
        cfg.update(self.ssl_overrides())
        return cfg

    def send_http_request(self, cert=None, verify=False, protocol='https'):
        # verify decides whether or not the client should verify the servers certificate
        return requests.post("{}://localhost:8200/intake/v2/events".format(protocol),
                             headers={'content-type': 'application/x-ndjson'},
                             data=self.get_event_payload(),
                             cert=cert,
                             verify=verify)

    def ssl_connect(self, protocol=ssl.PROTOCOL_TLSv1_2, ciphers=None):
        context = ssl.SSLContext(protocol)
        if ciphers:
            context.set_ciphers(ciphers)
        context.load_verify_locations(self.ca_cert)
        context.load_cert_chain(certfile=self.server_cert, keyfile=self.server_key, password=self.password)
        s = context.wrap_socket(ssl.socket())
        s.connect((self.host, self.port))


class TestSSLBadPassphraseTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_key_passphrase": "invalid"}

    @raises(TimeoutError)
    def setUp(self):
        super(TestSecureServerBaseTest, self).setUp()


@integration_test
class TestSSLEnabledNoClientVerificationTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_client_authentication": "none"}

    def test_https_no_cert_ok(self):
        r = self.send_http_request(verify=self.ca_cert)
        assert r.status_code == 202, r.status_code

    def test_http_fails(self):
        with self.assertRaises(Exception):
            self.send_http_request(protocol='http')

    @raises(SSLError)
    def test_https_server_validation_fails(self):
        r = self.send_http_request(verify=True)
        assert r.status_code == 202, r.status_code


@integration_test
class TestSSLEnabledOptionalClientVerificationTest(TestSecureServerBaseTest):
    # no ssl_overrides necessary as `optional` is default

    def test_https_no_certificate_ok(self):
        r = self.send_http_request(verify=self.ca_cert)
        assert r.status_code == 202, r.status_code

    @raises(SSLError)
    def test_https_verify_cert_if_given(self):
        self.send_http_request(verify=self.ca_cert,
                               cert=(self.simple_cert, self.simple_key))

    @raises(SSLError)
    def test_https_self_signed_cert(self):
        # CA is not configured server side, so self signed certs are not valid
        r = self.send_http_request(verify=self.ca_cert,
                                   cert=(self.client_cert, self.client_key))
        assert r.status_code == 202, r.status_code


@integration_test
class TestSSLEnabledOptionalClientVerificationWithCATest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_certificate_authorities": self.ca_cert}

    @raises(SSLError)
    def test_https_no_certificate(self):
        # since CA is configured, client auth is required
        r = self.send_http_request(verify=self.ca_cert)
        assert r.status_code == 202, r.status_code

    @raises(SSLError)
    def test_https_verify_cert_if_given(self):
        # invalid certificate
        self.send_http_request(verify=self.ca_cert,
                               cert=(self.simple_cert, self.simple_key))

    def test_https_auth_cert_ok(self):
        r = self.send_http_request(verify=self.ca_cert,
                                   cert=(self.client_cert, self.client_key))
        assert r.status_code == 202, r.status_code


@integration_test
class TestSSLEnabledRequiredClientVerificationTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_client_authentication": "required",
                "ssl_certificate_authorities": self.ca_cert}

    @raises(SSLError)
    def test_https_no_cert_fails(self):
        self.send_http_request(verify=self.ca_cert)

    @raises(SSLError)
    def test_https_invalid_cert_fails(self):
        self.send_http_request(verify=self.ca_cert,
                               cert=(self.simple_cert, self.simple_key))

    def test_https_auth_cert_ok(self):
        r = self.send_http_request(verify=self.ca_cert,
                                   cert=(self.client_cert, self.client_key))
        assert r.status_code == 202, r.status_code


@integration_test
class TestSSLDefaultSupportedProcotolsTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_certificate_authorities": self.ca_cert}

    @raises(ssl.SSLError)
    def test_tls_v1_0(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1)

    def test_tls_v1_1(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1_1)

    def test_tls_v1_2(self):
        self.ssl_connect()


@integration_test
class TestSSLSupportedProcotolsTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_supported_protocols": ["TLSv1.2"],
                "ssl_certificate_authorities": self.ca_cert}

    @raises(ssl.SSLError)
    def test_tls_v1_1(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1_1)

    def test_tls_v1_2(self):
        self.ssl_connect()


@integration_test
class TestSSLSupportedCiphersTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_cipher_suites": ['ECDHE-RSA-AES-128-GCM-SHA256'],
                "ssl_certificate_authorities": self.ca_cert}

    def test_https_no_cipher_set(self):
        self.ssl_connect()

    def test_https_supports_cipher(self):
        # set the same cipher in the client as set in the server
        self.ssl_connect(ciphers='ECDHE-RSA-AES128-GCM-SHA256')

    def test_https_unsupported_cipher(self):
        # client only offers unsupported cipher
        with self.assertRaisesRegexp(ssl.SSLError, 'SSLV3_ALERT_HANDSHAKE_FAILURE'):
            self.ssl_connect(ciphers='ECDHE-RSA-AES256-SHA384')

    def test_https_no_cipher_selected(self):
        # client provides invalid cipher
        with self.assertRaisesRegexp(ssl.SSLError, 'No cipher can be selected'):
            self.ssl_connect(ciphers='AES1sd28-CCM8')
