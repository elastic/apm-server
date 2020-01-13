import json
import unittest

import requests

from apmserver import ElasticTest, integration_test


@integration_test
class LoggingIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
    }

    def test_log_valid_event(self):
        with open(self.get_transaction_payload_path()) as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 202, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            self.assertDictContainsSubset({
                "level": "info",
                "message": "request accepted",
                "response_code": 202,
            }, req)

    def test_log_invalid_event(self):
        with open(self.get_payload_path("invalid-event.ndjson")) as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 400, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            self.assertDictContainsSubset({
                "level": "error",
                "message": "data validation error",
                "response_code": 400,
            }, req)
            error = req.get("error")
            assert error.startswith("error validating JSON document against schema:"), json.dumps(req)


@integration_test
class LoggingIntegrationEventSizeTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "max_event_size": "100",
    }

    def test_log_event_size_exceeded(self):
        with open(self.get_transaction_payload_path()) as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 400, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            self.assertDictContainsSubset({
                "level": "error",
                "message": "request body too large",
                "response_code": 400,
            }, req)
            error = req.get("error")
            assert error.startswith("event exceeded the permitted size."), json.dumps(req)


@integration_test
class LoggingIntegrationTraceCorrelationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "instrumentation_enabled": "true",
    }

    def test_trace_ids(self):
        with open(self.get_transaction_payload_path()) as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 202, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            self.assertIn("trace.id", req)
            self.assertIn("transaction.id", req)
            self.assertEqual(req["transaction.id"], req["request_id"])
