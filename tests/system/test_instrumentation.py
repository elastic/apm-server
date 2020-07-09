from datetime import datetime, timedelta
import os
import time
import requests

from apmserver import integration_test
from apmserver import ElasticTest
from test_auth import APIKeyBaseTest
from helper import wait_until
from es_helper import index_profile, index_transaction

# Set ELASTIC_APM_API_REQUEST_TIME to a short duration
# to speed up the time taken for self-tracing events
# to be ingested.
os.environ["ELASTIC_APM_API_REQUEST_TIME"] = "1s"


# Exercises the DEPRECATED apm-server.instrumentation.* config
# When updating this file, consider test_libbeat_instrumentation.py
# Remove in 8.0

def get_instrumentation_event(es, index):
    query = {"term": {"service.name": "apm-server"}}
    return es.count(index=index, body={"query": query})['count'] > 0


@integration_test
class TestInMemoryTracingAPIKey(APIKeyBaseTest):
    def config(self):
        cfg = super(TestInMemoryTracingAPIKey, self).config()
        cfg.update({
            "api_key_enabled": True,
            "instrumentation_enabled": "true",
        })
        return cfg

    def test_api_key_auth(self):
        """Self-instrumentation using in-memory listener without configuring an APIKey"""

        # Send a POST request to the intake API URL. Doesn't matter what the
        # request body contents are, as the request will fail due to lack of
        # authorization. We just want to trigger the server's in-memory tracing,
        # and test that the in-memory tracer works without having an api_key configured
        r = requests.post(self.intake_url, data="invalid")
        self.assertEqual(401, r.status_code)

        wait_until(lambda: get_instrumentation_event(self.es, index_transaction),
                   name='have in-memory instrumentation documents without api_key')


@integration_test
class TestExternalTracingAPIKey(APIKeyBaseTest):
    def config(self):
        cfg = super(TestExternalTracingAPIKey, self).config()
        api_key = self.create_apm_api_key([self.privilege_event], self.resource_any)
        cfg.update({
            "api_key_enabled": True,
            "instrumentation_enabled": "true",
            "instrumentation_api_key": api_key,
            # Set instrumentation.hosts to the same APM Server.
            #
            # Explicitly specifying hosts configures the tracer to
            # behave as if it's sending to an external server, rather
            # than using the in-memory transport that bypasses auth.
            "instrumentation_host": APIKeyBaseTest.host,
        })
        return cfg

    def test_api_key_auth(self):
        # Send a POST request to the intake API URL. Doesn't matter what the
        # request body contents are, as the request will fail due to lack of
        # authorization. We just want to trigger the server's tracing.
        r = requests.post(self.intake_url, data="invalid")
        self.assertEqual(401, r.status_code)

        wait_until(lambda: get_instrumentation_event(self.es, index_transaction),
                   name='have external server instrumentation documents with api_key')


@integration_test
class TestExternalTracingSecretToken(ElasticTest):
    def config(self):
        cfg = super(TestExternalTracingSecretToken, self).config()
        secret_token = "abc123"
        cfg.update({
            "secret_token": secret_token,
            "instrumentation_enabled": "true",
            "instrumentation_secret_token": secret_token,
            # Set instrumentation.hosts to the same APM Server.
            #
            # Explicitly specifying hosts configures the tracer to
            # behave as if it's sending to an external server, rather
            # than using the in-memory transport that bypasses auth.
            "instrumentation_host": ElasticTest.host,
        })
        return cfg

    def test_secret_token_auth(self):
        # Send a POST request to the intake API URL. Doesn't matter what the
        # request body contents are, as the request will fail due to lack of
        # authorization. We just want to trigger the server's tracing.
        r = requests.post(self.intake_url, data="invalid")
        self.assertEqual(401, r.status_code)

        wait_until(lambda: get_instrumentation_event(self.es, index_transaction),
                   name='have external server instrumentation documents with secret_token')


class ProfilingTest(ElasticTest):
    def metric_fields(self):
        metric_fields = set()
        rs = self.es.search(index=index_profile)
        for hit in rs["hits"]["hits"]:
            profile = hit["_source"]["profile"]
            metric_fields.update((k for (k, v) in profile.items() if type(v) is int))
        return metric_fields

    def wait_for_profile(self):
        def cond():
            response = self.es.count(index=index_profile, body={"query": {"term": {"processor.name": "profile"}}})
            return response['count'] != 0
        wait_until(cond, max_timeout=10, name="waiting for profile")


@integration_test
class TestCPUProfiling(ProfilingTest):
    config_overrides = {
        "instrumentation_enabled": "true",
        "profiling_cpu_enabled": "true",
        "profiling_cpu_interval": "1s",
        "profiling_cpu_duration": "5s",
    }

    def test_self_profiling(self):
        """CPU profiling enabled"""

        def create_load():
            payload_path = self.get_payload_path("transactions_spans.ndjson")
            with open(payload_path) as f:
                requests.post(self.intake_url, data=f, headers={'content-type': 'application/x-ndjson'})

        # Wait for profiling to begin, and then start sending data
        # to the server to create some CPU load.

        time.sleep(1)
        start = datetime.now()
        while datetime.now()-start < timedelta(seconds=5):
            create_load()
        self.wait_for_profile()

        expected_metric_fields = set([u"cpu.ns", u"samples.count", u"duration"])
        metric_fields = self.metric_fields()
        self.assertEqual(metric_fields, expected_metric_fields)


@integration_test
class TestHeapProfiling(ProfilingTest):
    config_overrides = {
        "instrumentation_enabled": "true",
        "profiling_heap_enabled": "true",
        "profiling_heap_interval": "1s",
    }

    def test_self_profiling(self):
        """Heap profiling enabled"""

        time.sleep(1)
        self.wait_for_profile()

        expected_metric_fields = set([
            u"alloc_objects.count",
            u"inuse_objects.count",
            u"alloc_space.bytes",
            u"inuse_space.bytes",
        ])
        metric_fields = self.metric_fields()
        self.assertEqual(metric_fields, expected_metric_fields)
