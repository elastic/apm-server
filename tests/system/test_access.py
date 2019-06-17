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
        with open(os.devnull, 'wb') as devnull:
            subprocess.call([cls.create_certs_cmd, cls.config_path, cls.cert_path,
                             "foobar"], stdout=devnull, stderr=devnull)
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
        self.ecdsa_cert = os.path.join(self.cert_path, "ecdsa.crt.pem")
        self.ecdsa_key = os.path.join(self.cert_path, "ecdsa.key.pem")
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

    def openssl_ciphers(self):
        ciphers = subprocess.Popen(["openssl", "ciphers"], stdout=subprocess.PIPE).stdout.read()
        return ciphers.split(':')

    def check_ciphers(self, expected_ciphers):
        allowed_ciphers = set()
        for cipher in self.openssl_ciphers():
            try:
                subprocess.check_call(["curl", "-k", "--ciphers", cipher, "--http1.1", "https://localhost:8200"],
                                      stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                allowed_ciphers.add(cipher)
            except subprocess.CalledProcessError:
                pass
        self.assertItemsEqual(allowed_ciphers, expected_ciphers,
                              "\nAllowed: {}, \nExpected: {}, \nMissing: {}\nAdditional: {}".format(
                                  allowed_ciphers, expected_ciphers, expected_ciphers-allowed_ciphers, allowed_ciphers-expected_ciphers))


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


default_rsa_ciphers = set([
    # current default rsa ciphers in golang stdlib
    # in openssl notation
    'ECDHE-RSA-DES-CBC3-SHA',  # TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
    'ECDHE-RSA-AES128-SHA',  # TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
    'ECDHE-RSA-AES128-GCM-SHA256',  # TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    'ECDHE-RSA-AES256-SHA',  # TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
    'ECDHE-RSA-AES256-GCM-SHA384',  # TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    'DES-CBC3-SHA',  # TLS_RSA_WITH_3DES_EDE_CBC_SHA
    'AES128-SHA',  # TLS_RSA_WITH_AES_128_CBC_SHA
    'AES128-GCM-SHA256',  # TLS_RSA_WITH_AES_128_GCM_SHA256
    'AES256-SHA',  # TLS_RSA_WITH_AES_256_CBC_SHA
    'AES256-GCM-SHA384',  # TLS_RSA_WITH_AES_256_GCM_SHA384
    'ECDHE-RSA-CHACHA20-POLY1305',  # TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
])

default_ecdsa_ciphers = set([
    # current default ecdsa ciphers in golang stdlib
    # in openssl notation
    'ECDHE-ECDSA-AES128-SHA',  # TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA
    'ECDHE-ECDSA-AES128-GCM-SHA256',  # TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    'ECDHE-ECDSA-AES256-SHA',  # TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA
    'ECDHE-ECDSA-AES256-GCM-SHA384',  # TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
    'ECDHE-ECDSA-CHACHA20-POLY1305',  # TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
])

enabled_rsa_ciphers = set([
    # currently defined rsa ciphers in libbeat implementation
    # in openssl notation
    'ECDHE-RSA-DES-CBC3-SHA',  # TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
    'ECDHE-RSA-AES128-SHA',  # TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
    'ECDHE-RSA-AES128-SHA256',  # TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256
    'ECDHE-RSA-AES128-GCM-SHA256',  # TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    'ECDHE-RSA-AES256-SHA',  # TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
    'ECDHE-RSA-AES256-GCM-SHA384',  # TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    'ECDHE-RSA-RC4-SHA',  # TLS_ECDHE_RSA_WITH_RC4_128_SHA
    'DES-CBC3-SHA',  # TLS_RSA_WITH_3DES_EDE_CBC_SHA
    'AES128-SHA',  # TLS_RSA_WITH_AES_128_CBC_SHA
    'AES128-SHA256',  # TLS_RSA_WITH_AES_128_CBC_SHA256
    'AES128-GCM-SHA256',  # TLS_RSA_WITH_AES_128_GCM_SHA256
    'AES256-SHA',  # TLS_RSA_WITH_AES_256_CBC_SHA
    'AES256-GCM-SHA384',  # TLS_RSA_WITH_AES_256_GCM_SHA384
    'RC4-SHA',  # TLS_RSA_WITH_RC4_128_SHA
    'ECDHE-RSA-CHACHA20-POLY1305',  # TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
])

enabled_ecdsa_ciphers = set([
    # currently defined ecdsa ciphers in libbeat implementation
    # in openssl notation
    'ECDHE-ECDSA-AES128-SHA',  # TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA
    'ECDHE-ECDSA-AES128-SHA256',  # TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256
    'ECDHE-ECDSA-AES128-GCM-SHA256',  # TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    'ECDHE-ECDSA-AES256-SHA',  # TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA
    'ECDHE-ECDSA-AES256-GCM-SHA384',  # TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
    'ECDHE-ECDSA-RC4-SHA',  # TLS_ECDHE_ECDSA_WITH_RC4_128_SHA
    'ECDHE-ECDSA-CHACHA20-POLY1305',  # TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
])

defined_ciphers = [
    # currently defined ciphers in libbeat implementation
    # in libbeat notation
    "ECDHE-ECDSA-CHACHA20-POLY1305",
    "ECDHE-RSA-CHACHA20-POLY1205",
    "ECDHE-ECDSA-AES-256-GCM-SHA384",
    "ECDHE-ECDSA-AES-128-GCM-SHA256",
    "ECDHE-RSA-AES-128-GCM-SHA256",
    "ECDHE-RSA-AES-256-GCM-SHA384",
    "ECDHE-ECDSA-AES-128-CBC-SHA",
    "ECDHE-ECDSA-AES-128-CBC-SHA256",
    "ECDHE-RSA-AES-128-CBC-SHA",
    "ECDHE-RSA-AES-128-CBC-SHA256",
    "ECDHE-ECDSA-AES-256-CBC-SHA",
    "ECDHE-RSA-AES-256-CBC-SHA",
    "ECDHE-ECDSA-RC4-128-SHA",
    "ECDHE-RSA-3DES-CBC3-SHA",
    "ECDHE-RSA-RC4-128-SHA",
    "RSA-AES-256-GCM-SHA384",
    "RSA-RC4-128-SHA",
    "RSA-3DES-CBC3-SHA",
    "RSA-AES-128-CBC-SHA",
    "RSA-AES-128-CBC-SHA256",
    "RSA-AES-128-GCM-SHA256",
    "RSA-AES-256-CBC-SHA"
]


class TestDefaultRSACiphers(TestSecureServerBaseTest):
    def test_default_ciphers(self):
        self.check_ciphers(default_rsa_ciphers)


class TestDefaultECDSACiphers(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {
            "ssl_certificate": self.ecdsa_cert,
            "ssl_key": self.ecdsa_key,
            "ssl_client_authentication": "none"
        }

    def test_default_ciphers(self):
        self.check_ciphers(default_ecdsa_ciphers)


class TestEnabledRSACiphers(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_cipher_suites": defined_ciphers,
                "ssl_supported_protocols": ["SSLv3.0", "TLSv1.0", "TLSv1.1", "TLSv1.2"]}

    def test_enabled_ciphers(self):
        self.check_ciphers(enabled_rsa_ciphers)


class TestEnabledECDSACiphers(TestSecureServerBaseTest):
    def ssl_overrides(self):
        return {
            "ssl_certificate": self.ecdsa_cert,
            "ssl_key": self.ecdsa_key,
            "ssl_cipher_suites": defined_ciphers
        }

    def test_default_ciphers(self):
        self.check_ciphers(enabled_ecdsa_ciphers)
