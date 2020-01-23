import os
import re
import subprocess

from apmserver import integration_test, ElasticTest
from helper import wait_until


@integration_test
class Test(ElasticTest):
    def setUp(self):
        super(Test, self).setUp()
        wait_until(lambda: self.log_contains("Listening for Jaeger HTTP"), name="Jaeger HTTP listener started")
        wait_until(lambda: self.log_contains("Listening for Jaeger gRPC"), name="Jaeger gRPC listener started")

        # Extract the Jaeger server addresses.
        log = self.get_log()
        match = re.search("Listening for Jaeger HTTP requests on: (.*)$", log, re.MULTILINE)
        self.jaeger_http_url = "http://{}/{}".format(match.group(1), 'api/traces')
        match = re.search("Listening for Jaeger gRPC requests on: (.*)$", log, re.MULTILINE)
        self.jaeger_grpc_addr = match.group(1)

    def config(self):
        cfg = super(Test, self).config()
        cfg.update({
            "jaeger_grpc_enabled": "true",
            "jaeger_http_enabled": "true",
            # Listen on dynamic ports
            "jaeger_grpc_host": "localhost:0",
            "jaeger_http_host": "localhost:0",
        })
        return cfg

    def test_jaeger_http(self):
        """
        This test sends a Jaeger span in Thrift encoding over HTTP, and verifies that it is indexed.
        """
        jaeger_span_thrift = self.get_testdata_path('jaeger', 'span.thrift')
        self.load_docs_with_template(jaeger_span_thrift, self.jaeger_http_url, 'transaction', 1,
                                     extra_headers={"content-type": "application/vnd.apache.thrift.binary"})

        self.assert_no_logged_warnings()
        transaction_docs = self.wait_for_events('transaction', 1)
        self.approve_docs('jaeger_span', transaction_docs)

    def test_jaeger_grpc(self):
        """
        This test sends a Jaeger batch over gRPC, and verifies that the spans are indexed.
        """
        jaeger_request_data = self.get_testdata_path('jaeger', 'batch_0.json')

        client = os.path.join(os.path.dirname(__file__), 'jaegergrpc')
        subprocess.check_call(['go', 'run', client,
                               '-addr', self.jaeger_grpc_addr,
                               '-insecure',
                               jaeger_request_data,
                               ])

        self.assert_no_logged_warnings()
        transaction_docs = self.wait_for_events('transaction', 1)
        error_docs = self.wait_for_events('error', 3)
        self.approve_docs('jaeger_batch_0', transaction_docs + error_docs)
