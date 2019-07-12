import json
import unittest

import requests

from apmserver import ElasticTest
from beat.beat import INTEGRATION_TESTS

from apmserver import ElasticTest


class LoggingIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
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
                "message": "handled request",
                "response_code": 202,
            }, req)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
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
                "message": "error handling request",
                "response_code": 400,
            }, req)
            error = req.get("error")
            assert error.startswith("error validating JSON document against schema:"), json.dumps(req)


class LoggingIntegrationEventSizeTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "max_event_size": "100",
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
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
                "message": "error handling request",
                "response_code": 400,
            }, req)
            error = req.get("error")
            assert error.startswith("event exceeded the permitted size."), json.dumps(req)
