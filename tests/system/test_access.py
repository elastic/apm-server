from apmserver import ServerBaseTest, ServerSetUpBaseTest

import requests
import ssl
import os
import subprocess
import shutil
import unittest
import json
import random
import string
import base64
import time

from nose.tools import raises
from requests.exceptions import SSLError, ChunkedEncodingError
from beat.beat import INTEGRATION_TESTS, TimeoutError
from requests.packages.urllib3.exceptions import SubjectAltNameWarning
requests.packages.urllib3.disable_warnings(SubjectAltNameWarning)

INTEGRATION_TESTS = os.environ.get('INTEGRATION_TESTS', False)


def headers(auth, content_type='application/x-ndjson'):
    h = {'content-type': content_type}
    if auth != "":
        auth_headers = {'Authorization': auth}
        auth_headers.update(h)
        return auth_headers
    return h


class BaseAPIKeySetup(ServerBaseTest):
    @classmethod
    def setUpClass(cls):
        # According to https://docs.python.org/2/library/unittest.html#setupclass-and-teardownclass setUp and tearDown
        # should be skipped when class is skipped, which is apparently not true.
        # This is a hack to avoid running the setup while it should be skipped
        if not INTEGRATION_TESTS:
            return
        # cls.cmd = os.path.abspath(os.path.join(os.path.dirname(__file__), "config", "api_key_setup.sh"))
        # with open(os.devnull, 'wb') as dev_null:
        #     subprocess.call([cls.cmd, "apm_server_user", "changeme"])

        # apm-backend application
        cls.application_backend = "apm-backend"

        # apm-backend privileges
        cls.privilege_access = "action:access"
        cls.privilege_intake = "action:intake"
        cls.privilege_config = "action:config"
        cls.privilege_sourcemap = "action:sourcemap"
        cls.privilege_full = "action:full"
        cls.privileges = [cls.privilege_access, cls.privilege_intake, cls.privilege_config, cls.privilege_sourcemap]
        cls.api_key_ids = []

        content_type = 'application/json'
        resources = ["*"]
        cls.user_api_keys = "apm_server_user"
        es_url = cls.get_elasticsearch_url(user=cls.user_api_keys)

        # create privileges
        url_privileges = "{}/_security/privilege".format(es_url)
        payload = json.dumps({cls.application_backend: {
            "access": {"actions": [cls.privilege_access]},
            "sourcemap": {"actions": [cls.privilege_sourcemap]},
            "intake": {"actions": [cls.privilege_intake]},
            "config": {"actions": [cls.privilege_config]},
            "full": {"actions": [cls.privilege_full]}}})
        resp = requests.put(url_privileges, data=payload, headers=headers("", content_type=content_type))
        assert resp.status_code == 200, resp.status_code

        # ensure user that creates the api keys has all privileges
        # see how application privileges can be added to user roles in testing/docker/elasticsearch/roles.yml
        url_user_privileges = "{}/_security/user/_has_privileges".format(es_url)
        payload = json.dumps({"application": [{
            "application": cls.application_backend,
            "privileges": cls.privileges + [cls.privilege_full],
            "resources": resources}]})
        resp = requests.post(url_user_privileges, data=payload, headers=headers("", content_type=content_type))
        assert resp.status_code == 200, resp.status_code
        assert "has_all_requested" in resp.json(), resp.content
        assert resp.json()["has_all_requested"] == True, resp.json()

        # create api keys
        def random_str():
            return "".join(random.choice(string.ascii_letters) for i in range(16))

        def create_api_key(privileges):
            url = "{}/_security/api_key".format(es_url)
            name = "apm-{}".format(random_str())
            payload = json.dumps({
                "name": name,
                "role_descriptors": {
                    name+"role_desc": {
                        "applications": [
                            {"application": cls.application_backend, "privileges": privileges, "resources": resources}]}}})
            resp = requests.post(url, data=payload, headers=headers("", content_type=content_type))
            assert resp.status_code == 200, resp.status_code
            cls.api_key_ids += [resp.json()["id"]]
            return "ApiKey {}".format(base64.b64encode("{}:{}".format(resp.json()["id"], resp.json()["api_key"])))

        # authorized for: cls.privilege_access, cls.privilege_intake, cls.privilege_config, cls.privilege_sourcemap
        cls.api_key_privileges_apm = create_api_key(cls.privileges)

        # authorized for: cls.privilege_full
        cls.api_key_privilege_full = create_api_key([cls.privilege_full])

        # authorized for: no access only
        cls.api_key_privilege_access = create_api_key([cls.privilege_access])

        # authorized for: not existing privilege
        cls.api_key_missing_privilege = create_api_key(["action:not_existing"])

        super(BaseAPIKeySetup, cls).setUpClass()

    @classmethod
    def tearDownClass(cls):
        if not INTEGRATION_TESTS:
            return

        for id in cls.api_key_ids:
            url = "{}/_security/api_key".format(cls.get_elasticsearch_url(user=cls.user_api_keys))
            payload = json.dumps({'id': id})
            resp = requests.delete(url, data=payload, headers=headers("", content_type='application/json'))
            assert resp.status_code == 200, resp.status_code

        super(BaseAPIKeySetup, cls).tearDownClass()

    def setUp(self):
        self.authorized_keys = ["Bearer 1234", self.api_key_privileges_apm, self.api_key_privilege_full]
        self.unauthorized_keys = ['', 'Bearer ', 'Bearer wrongtoken', 'Wrongbearer 1234',
                                  self.api_key_missing_privilege,
                                  "ApiKey nonexisting"]
        super(BaseAPIKeySetup, self).setUp()


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class TestAccessWithAuthorizationAPIKeyDisabled(BaseAPIKeySetup):

    def config(self):
        cfg = super(TestAccessWithAuthorizationAPIKeyDisabled, self).config()
        cfg.update({"auth_bearer_token": "1234"})
        return cfg

    def test_no_access_with_api_key(self):
        """
        Test that authorized API Key is not accepted when API Key usage is disabled
        """

        url = 'http://localhost:8200/intake/v2/events'
        events = self.get_event_payload()

        # access without token denied
        resp = requests.post(url, data=events, headers=headers(""))
        assert resp.status_code == 401, resp.status_code

        # access with valid Bearer token allowed
        resp = requests.post(url, data=events, headers=headers("Bearer 1234"))
        assert resp.status_code == 202, resp.status_code

        # access with generally valid API Key denied
        resp = requests.post(url, data=events, headers=headers(self.api_key_privileges_apm))
        assert resp.status_code == 401, resp.status_code


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class TestAPIKeyCache(BaseAPIKeySetup):
    def config(self):
        cfg = super(TestAPIKeyCache, self).config()
        cfg.update({"api_key": True,
                    "api_key_cache_size": 4})
        return cfg

    def test_cache_full(self):
        """
        Test that authorized API Key is not accepted when cache is full
        """
        url = 'http://localhost:8200/intake/v2/events'
        events = self.get_event_payload()

        def assert_authorized(api_key):
            resp = requests.post(url, data=events, headers=headers(api_key))
            assert resp.status_code != 401,  "token: {}, status_code: {}".format(api_key, resp.status_code)

        def assert_unauthorized(api_key):
            resp = requests.post(url, data=events, headers=headers(api_key))
            assert resp.status_code == 401,  "token: {}, status_code: {}".format(api_key, resp.status_code)

        # send request with 2 unauthorized api keys
        assert_unauthorized(self.api_key_privilege_access)
        assert_unauthorized(self.api_key_missing_privilege)
        # send authorized api keys: the second one reaches a full cache
        assert_authorized(self.api_key_privilege_full)
        assert_unauthorized(self.api_key_privileges_apm)
        assert_authorized(self.api_key_privilege_full)


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class TestAccessWithAuthorization(BaseAPIKeySetup):
    def config(self):
        cfg = super(TestAccessWithAuthorization, self).config()
        cfg.update({"auth_bearer_token": "1234",
                    "api_key": True,
                    "enable_rum": True,
                    "rum_rate_limit": 1000,
                    "kibana_enabled": True})
        return cfg

    def test_root(self):
        """
        Test authorization logic for root endpoint
        """
        url = 'http://localhost:8200/'

        for token in self.unauthorized_keys:
            resp = requests.get(url, headers=headers(token))
            assert resp.status_code == 200, "token: {}, status_code: {}".format(token, resp.status_code)
            assert resp.content == '', "token: {}, response: {}".format(token, resp.content)

        for token in self.authorized_keys+[self.api_key_privilege_access]:
            resp = requests.get(url, headers=headers(token))
            assert resp.status_code == 200,  "token: {}, status_code: {}".format(token, resp.status_code)
            assert resp.content != '', token
            for token in ["build_date", "build_sha", "version"]:
                assert token in resp.json(), "token: {}, response: {}".format(token, resp.content)

    def test_backend_intake(self):
        """
        Test authorization logic for backend Intake endpoint
        """
        url = 'http://localhost:8200/intake/v2/events'
        events = self.get_event_payload()

        for token in self.authorized_keys:
            resp = requests.post(url, data=events, headers=headers(token))
            assert resp.status_code == 202,  "token: {}, status_code: {}".format(token, resp.status_code)

        for token in self.unauthorized_keys:
            resp = requests.post(url, data=events, headers=headers(token))
            assert resp.status_code == 401,  "token: {}, status_code: {}".format(token, resp.status_code)

    def test_rum_intake(self):
        """
        Test authorization logic for RUM Intake endpoint.
        """
        url = 'http://localhost:8200/intake/v2/rum/events'
        events = self.get_event_payload()

        # Endpoint is not secured, all keys are expected to be allowed.
        for token in self.authorized_keys + self.unauthorized_keys:
            resp = requests.post(url, data=events, headers=headers(token))
            assert resp.status_code == 202,  "token: {}, status_code: {}".format(token, resp.status_code)

    def test_agent_config(self):
        """
        Test authorization logic for backend Agent Configuration endpoint
        """
        url = self.agent_config_url

        for token in self.authorized_keys:
            resp = requests.get(url, headers=headers(token, content_type="application/json"))
            assert resp.status_code != 401,  "token: {}, status_code: {}".format(token, resp.status_code)

        for token in self.unauthorized_keys:
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
            url = 'http://localhost:8200/assets/v1/sourcemaps'
            f = open(self._beat_path_join('testdata', 'sourcemap', 'bundle_no_mapping.js.map'))
            resp = requests.post(url,
                                 headers=headers(token, content_type="application/json"),
                                 files={'sourcemap': f},
                                 data={'service_version': '1.0.1',
                                       'bundle_filepath': 'mapping.js.map',
                                       'service_name': 'apm-agent-js'
                                       })
            return resp

        for token in self.unauthorized_keys:
            resp = upload(token)
            assert resp.status_code == 401, "token: {}, status_code: {}".format(token, resp.status_code)

        for token in self.authorized_keys:
            resp = upload(token)
            assert resp.status_code != 401, "token: {}, status_code: {}".format(token, resp.status_code)


class TestAccessWithSecretToken(ServerBaseTest):
    def config(self):
        cfg = super(TestAccessWithSecretToken, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg

    def test_backend_intake(self):
        """
        Test that access works with token
        """

        url = 'http://localhost:8200/intake/v2/events'
        events = self.get_event_payload()

        r = requests.post(url, data=events, headers=headers(""))
        assert r.status_code == 401, r.status_code

        r = requests.post(url, data=events, headers=headers('Bearer 1234'))
        assert r.status_code == 202, r.status_code


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class TestSecureServerBaseTest(ServerSetUpBaseTest):
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
        super(TestSecureServerBaseTest, self).setUp()

    def tearDown(self):
        super(TestSecureServerBaseTest, self).tearDown()
        self.apmserver_proc.kill_and_wait()

    def config_overrides(self):
        cfg = {
            "ssl_enabled": "true",
            "ssl_certificate": self.server_cert,
            "ssl_key": self.server_key,
            "ssl_key_passphrase": self.password
        }
        cfg.update(self.ssl_overrides())
        return cfg

    def ssl_overrides(self):
        return {}

    def config(self):
        cfg = super(TestSecureServerBaseTest, self).config()
        cfg.update(self.config_overrides())
        self.host = "localhost"
        self.port = 8200
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


class TestSSLSupportedProcotolsTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_supported_protocols": ["TLSv1.2"],
                "ssl_certificate_authorities": self.ca_cert}

    @raises(ssl.SSLError)
    def test_tls_v1_1(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1_1)

    def test_tls_v1_2(self):
        self.ssl_connect()


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
