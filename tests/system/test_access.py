from apmserver import ServerBaseTest, ServerSetUpBaseTest

import requests
import ssl
import os
import subprocess
import shutil

from nose.tools import raises
from requests.exceptions import SSLError, ChunkedEncodingError
from requests.packages.urllib3.exceptions import SubjectAltNameWarning
requests.packages.urllib3.disable_warnings(SubjectAltNameWarning)


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


class SecureServerBaseTest(ServerSetUpBaseTest):
    @classmethod
    def setUpClass(cls):
        cls.config_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "config"))
        cls.cert_path = os.path.join(cls.config_path, "certs")
        shutil.rmtree(cls.cert_path, ignore_errors=True)
        cls.create_certs_cmd = os.path.join(cls.config_path, "create_certs.sh")
        with open(os.devnull, 'wb') as dev_null:
            subprocess.call([cls.create_certs_cmd, cls.config_path, cls.cert_path], stdout=dev_null, stderr=dev_null)
        super(SecureServerBaseTest, cls).setUpClass()

    @classmethod
    def tearDownClass(cls):
        super(SecureServerBaseTest, cls).tearDownClass()
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
        super(SecureServerBaseTest, self).setUp()

    def tearDown(self):
        super(SecureServerBaseTest, self).tearDown()
        self.apmserver_proc.kill_and_wait()

    def config_overrides(self):
        cfg = {
            "ssl_enabled": "true",
            "ssl_certificate_authorities": self.ca_cert,
            "ssl_certificate": self.server_cert,
            "ssl_key": self.server_key
        }
        cfg.update(self.ssl_overrides())
        return cfg

    def ssl_overrides(self):
        return {}

    def config(self):
        cfg = super(SecureServerBaseTest, self).config()
        cfg.update(self.config_overrides())
        self.host = "localhost"
        self.port = 8200
        return cfg


class TestSSLEnabledNoClientVerification(SecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_client_authentication": "none"}

    def test_https_no_cert_ok(self):
        r = requests.post("https://localhost:8200/intake/v2/events",
                          headers={'content-type': 'application/x-ndjson'},
                          data=self.get_event_payload(),
                          verify=self.ca_cert)
        assert r.status_code == 202, r.status_code

    @raises(ChunkedEncodingError)
    def test_http_fails(self):
        requests.post("http://localhost:8200/intake/v2/events",
                      headers={'content-type': 'application/x-ndjson'},
                      data=self.get_event_payload())


class TestSSLEnabledOptionalClientVerification(SecureServerBaseTest):
    def ssl_overrides(self):
        return {"ssl_client_authentication": "optional"}

    def test_https_no_certificate_ok(self):
        r = requests.post("https://localhost:8200/intake/v2/events",
                          headers={'content-type': 'application/x-ndjson'},
                          data=self.get_event_payload(),
                          verify=self.ca_cert)
        assert r.status_code == 202, r.status_code

    @raises(SSLError)
    def test_https_verify_cert_if_given(self):
        requests.post("https://localhost:8200/intake/v2/events",
                      headers={'content-type': 'application/x-ndjson'},
                      data=self.get_event_payload(),
                      cert=(self.simple_cert, self.simple_key),
                      verify=self.ca_cert)

    def test_https_auth_cert_ok(self):
        requests.post("https://localhost:8200/intake/v2/events",
                      headers={'content-type': 'application/x-ndjson'},
                      data=self.get_event_payload(),
                      cert=(self.client_cert, self.client_key),
                      verify=self.ca_cert)


class TestSSLEnabledRequiredClientVerification(SecureServerBaseTest):
    # no ssl_overrides necessary as `required` is default

    @raises(SSLError)
    def test_https_no_cert_fails(self):
        requests.post("https://localhost:8200/intake/v2/events",
                      headers={'content-type': 'application/x-ndjson'},
                      data=self.get_event_payload(),
                      verify=self.ca_cert)

    @raises(SSLError)
    def test_https_invalid_cert_fails(self):
        requests.post("https://localhost:8200/intake/v2/events",
                      headers={'content-type': 'application/x-ndjson'},
                      data=self.get_event_payload(),
                      cert=(self.simple_cert, self.simple_key),
                      verify=self.ca_cert)

    def test_https_auth_cert_ok(self):
        requests.post("https://localhost:8200/intake/v2/events",
                      headers={'content-type': 'application/x-ndjson'},
                      data=self.get_event_payload(),
                      cert=(self.client_cert, self.client_key),
                      verify=self.ca_cert)


class TestSSLDefaultSupportedProcotols(SecureServerBaseTest):

    def ssl_overrides(self):
        return {"ssl_client_authentication": "optional"}

    @raises(ssl.SSLError)
    def test_tls_v1_0(self):
        context = ssl.SSLContext(ssl.PROTOCOL_TLSv1)
        s = context.wrap_socket(ssl.socket())
        s.connect((self.host, self.port))

    def test_tls_v1_1(self):
        context = ssl.SSLContext(ssl.PROTOCOL_TLSv1_1)
        s = context.wrap_socket(ssl.socket())
        s.connect((self.host, self.port))

    def test_tls_v1_2(self):
        context = ssl.SSLContext(ssl.PROTOCOL_TLSv1_2)
        s = context.wrap_socket(ssl.socket())
        s.connect((self.host, self.port))


class TestSSLSupportedProcotols(SecureServerBaseTest):

    def ssl_overrides(self):
        return {"ssl_client_authentication": "optional",
                "ssl_supported_protocols": ["TLSv1.2"]}

    @raises(ssl.SSLError)
    def test_tls_v1_1(self):
        context = ssl.SSLContext(ssl.PROTOCOL_TLSv1_1)
        s = context.wrap_socket(ssl.socket())
        s.connect((self.host, self.port))

    def test_tls_v1_2(self):
        context = ssl.SSLContext(ssl.PROTOCOL_TLSv1_2)
        s = context.wrap_socket(ssl.socket())
        s.connect((self.host, self.port))
