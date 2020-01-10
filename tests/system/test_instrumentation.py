from datetime import datetime, timedelta
import time

import requests

from apmserver import integration_test
from apmserver import ElasticTest


@integration_test
class ProfileIntegrationTest(ElasticTest):
    def metric_fields(self):
        metric_fields = set()
        rs = self.es.search(index=self.index_profile)
        for hit in rs["hits"]["hits"]:
            profile = hit["_source"]["profile"]
            metric_fields.update((k for (k, v) in profile.items() if type(v) is int))
        return metric_fields

    def wait_for_profile(self):
        def cond():
            self.es.indices.refresh(index=self.index_profile)
            response = self.es.count(index=self.index_profile, body={"query": {"term": {"processor.name": "profile"}}})
            return response['count'] != 0
        self.wait_until(cond, max_timeout=10, name="waiting for profile")


@integration_test
class CPUProfileIntegrationTest(ProfileIntegrationTest):
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
class HeapProfileIntegrationTest(ProfileIntegrationTest):
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
