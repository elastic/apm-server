import os
import re
import subprocess
from urllib.parse import urljoin
import requests

from apmserver import integration_test, ElasticTest
from helper import wait_until


class JaegerBaseTest(ElasticTest):
    def setUp(self):
        super(JaegerBaseTest, self).setUp()
        wait_until(lambda: self.log_contains("Listening for Jaeger HTTP"), name="Jaeger HTTP listener started")
        wait_until(lambda: self.log_contains("Listening for Jaeger gRPC"), name="Jaeger gRPC listener started")

        # Extract the Jaeger server addresses.
        log = self.get_log()
        match = re.search("Listening for Jaeger HTTP requests on: (.*)$", log, re.MULTILINE)
        self.jaeger_http_url = "http://{}/{}".format(match.group(1), 'api/traces')
        match = re.search("Listening for Jaeger gRPC requests on: (.*)$", log, re.MULTILINE)
        self.jaeger_grpc_addr = match.group(1)

    def config(self):
        cfg = super(JaegerBaseTest, self).config()
        cfg.update({
            "jaeger_grpc_enabled": "true",
            "jaeger_http_enabled": "true",
            # Listen on dynamic ports
            "jaeger_grpc_host": "localhost:0",
            "jaeger_http_host": "localhost:0",
            # jaeger_grpc_auth_tag is set in the base suite so we can
            # check that the authorization tag is always removed,
            # even if there's no secret token / API Key auth.
            "jaeger_grpc_auth_tag": "authorization",
            "logging_ecs": "false",
            "logging_json": "false",
        })
        return cfg


@integration_test
class Test(JaegerBaseTest):
    def test_jaeger_http(self):
        """
        This test sends a Jaeger span in Thrift encoding over HTTP, and verifies that it is indexed.
        """
        jaeger_span_thrift = self.get_testdata_path('jaeger', 'span.thrift')
        self.load_docs_with_template(jaeger_span_thrift, self.jaeger_http_url, 'transaction', 1,
                                     extra_headers={"content-type": "application/vnd.apache.thrift.binary"},
                                     file_mode="rb")

        self.assert_no_logged_warnings()
        transaction_docs = self.wait_for_events('transaction', 1)
        self.approve_docs('jaeger_span', transaction_docs)

    def test_jaeger_auth_tag_removed(self):
        """
        This test sends a Jaeger batch over gRPC, with an "authorization" process tag,
        and verifies that the spans are indexed without that process tag indexed as a label.
        """
        jaeger_request_data = self.get_testdata_path('jaeger', 'batch_0_authorization.json')

        client = os.path.join(os.path.dirname(__file__), 'jaegergrpc')
        subprocess.run(
            ['go', 'run', client, '-addr', self.jaeger_grpc_addr, '-insecure', jaeger_request_data],
            check=True,
        )

        transaction_docs = self.wait_for_events('transaction', 1)
        error_docs = self.wait_for_events('error', 3)
        self.approve_docs('jaeger_batch_0_auth_tag_removed', transaction_docs + error_docs)


@integration_test
class TestAuthTag(JaegerBaseTest):
    def config(self):
        cfg = super(TestAuthTag, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg

    def test_jaeger_unauthorized(self):
        """
        This test sends a Jaeger batch over gRPC, without an "authorization" process tag,
        and verifies that the spans are indexed.
        """
        jaeger_request_data = self.get_testdata_path('jaeger', 'batch_0.json')

        client = os.path.join(os.path.dirname(__file__), 'jaegergrpc')
        proc = subprocess.Popen(
            ['go', 'run', client, '-addr', self.jaeger_grpc_addr, '-insecure', jaeger_request_data],
            stderr=subprocess.PIPE,
        )
        stdout, stderr = proc.communicate()
        self.assertNotEqual(proc.returncode, 0)
        self.assertRegex(stderr.decode("utf-8"), "Unauthenticated")

    def test_jaeger_authorized(self):
        """
        This test sends a Jaeger batch over gRPC, with an "authorization" process tag,
        and verifies that the spans are indexed without that tag indexed as a label.
        """
        jaeger_request_data = self.get_testdata_path('jaeger', 'batch_0_authorization.json')

        client = os.path.join(os.path.dirname(__file__), 'jaegergrpc')
        subprocess.run(
            ['go', 'run', client, '-addr', self.jaeger_grpc_addr, '-insecure', jaeger_request_data],
            check=True,
        )

        transaction_docs = self.wait_for_events('transaction', 1)
        error_docs = self.wait_for_events('error', 3)
        self.approve_docs('jaeger_batch_0_authorization', transaction_docs + error_docs)


@integration_test
class GRPCSamplingTest(JaegerBaseTest):

    def config(self):
        cfg = super(GRPCSamplingTest, self).config()
        cfg.update({
            "jaeger_grpc_sampling_enabled": "true",
            "kibana_host": self.get_kibana_url(),
            "kibana_enabled": "true",
            "acm_cache_expiration": "1s"
        })
        cfg.update(self.config_overrides)
        return cfg

    def create_service_config(self, service, sampling_rate, agent=None):
        return self.kibana.create_or_update_agent_config(
            service, {"transaction_sample_rate": "{}".format(sampling_rate)}, agent=agent)

    def call_sampling_endpoint(self, service):
        client = os.path.join(os.path.dirname(__file__), 'jaegergrpc')
        out = os.path.abspath(self.working_dir) + "/sampling_response"
        subprocess.check_call(['go', 'run', client,
                               '-addr', self.jaeger_grpc_addr,
                               '-insecure',
                               '-endpoint', "sampler",
                               '-service', service,
                               '-out', out
                               ])
        with open(out, "r") as out:
            return out.read()

    def test_jaeger_grpc_sampling(self):
        """
        This test sends Jaeger sampling requests over gRPC, and tests responses
        """

        # test returns a configured default sampling strategy
        service = "all"
        sampling_rate = 0.35
        self.create_service_config(service, sampling_rate)
        expected = "strategy: PROBABILISTIC, sampling rate: {}".format(sampling_rate)
        logged = self.call_sampling_endpoint(service)
        assert expected == logged, logged

        # test returns a configured sampling strategy
        service = "jaeger-service"
        sampling_rate = 0.75
        self.create_service_config(service, sampling_rate, agent="Jaeger/Ruby")
        expected = "strategy: PROBABILISTIC, sampling rate: {}".format(sampling_rate)
        logged = self.call_sampling_endpoint(service)
        assert expected == logged, logged

        # test returns an error as configured sampling strategy is not for Jaeger
        service = "foo"
        sampling_rate = 0.13
        self.create_service_config(service, sampling_rate, agent="Non-Jaeger/Agent")
        expected = "no sampling rate available, check server logs for more details"
        logged = self.call_sampling_endpoint(service)
        assert expected in logged, logged
