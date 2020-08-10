import json
import requests

from apmserver import ElasticTest, ServerBaseTest, integration_test, is_subset


@integration_test
class LoggingIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
    }

    def test_log_valid_event(self):
        with open(self.get_transaction_payload_path(), 'rb') as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 202, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            subset = {
                "level": "info",
                "message": "request accepted",
                "response_code": 202,
            }
            assert is_subset(subset, req)

    def test_log_invalid_event(self):
        with open(self.get_payload_path("invalid-event.ndjson"), 'rb') as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 400, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            subset = {
                "level": "error",
                "message": "data validation error",
                "response_code": 400,
            }
            assert is_subset(subset, req)
            error = req.get("error")
            assert error.startswith("failed to validate transaction: error validating JSON:"), json.dumps(req)


@integration_test
class LoggingIntegrationEventSizeTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "max_event_size": "100",
    }

    def test_log_event_size_exceeded(self):
        with open(self.get_transaction_payload_path(), 'rb') as f:
            r = requests.post(self.intake_url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
            assert r.status_code == 400, r.status_code
            intake_request_logs = list(self.logged_requests())
            assert len(intake_request_logs) == 1, "multiple requests found"
            req = intake_request_logs[0]
            subset = {
                "level": "error",
                "message": "request body too large",
                "response_code": 400,
            }
            assert is_subset(subset, req)
            error = req.get("error")
            assert error.startswith("event exceeded the permitted size."), json.dumps(req)
