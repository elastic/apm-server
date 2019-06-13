from apmserver import ServerBaseTest, ServerSetUpBaseTest

import requests
import ssl
import os
import subprocess
import shutil
import unittest

from nose.tools import raises
from requests.exceptions import SSLError, ChunkedEncodingError
from beat.beat import INTEGRATION_TESTS, TimeoutError
from requests.packages.urllib3.exceptions import SubjectAltNameWarning
requests.packages.urllib3.disable_warnings(SubjectAltNameWarning)

INTEGRATION_TESTS = os.environ.get('INTEGRATION_TESTS', False)


class TestAccessWithCredentials(ServerBaseTest):

    def config(self):
        cfg = super(TestAccessWithCredentials, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg

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
            "ssl_certificate_authorities": self.ca_cert,
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

    def test_https_auth_cert_ok(self):
        r = self.send_http_request(verify=self.ca_cert,
                                   cert=(self.client_cert, self.client_key))
        assert r.status_code == 202, r.status_code


class TestSSLEnabledRequiredClientVerificationTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_client_authentication": "required"}

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
    @raises(ssl.SSLError)
    def test_tls_v1_0(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1)

    def test_tls_v1_1(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1_1)

    def test_tls_v1_2(self):
        self.ssl_connect()


class TestSSLSupportedProcotolsTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_supported_protocols": ["TLSv1.2"]}

    @raises(ssl.SSLError)
    def test_tls_v1_1(self):
        self.ssl_connect(protocol=ssl.PROTOCOL_TLSv1_1)

    def test_tls_v1_2(self):
        self.ssl_connect()


class TestSSLSupportedCiphersTest(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_cipher_suites": ['ECDHE-RSA-AES128-GCM-SHA256']}

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
