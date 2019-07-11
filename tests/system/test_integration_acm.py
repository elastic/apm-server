import unittest

import requests

from apmserver import ElasticTest
from beat.beat import INTEGRATION_TESTS

from apmserver import ElasticTest


class AgentConfigurationKibanaDownIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "secret_token": "supersecret",
        "kibana_enabled": "true",
        "kibana_host": "unreachablehost"
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_log_config_request(self):
        r1 = requests.get(self.agent_config_url,
                          headers={
                              "Content-Type": "application/x-ndjson",
                          })
        assert r1.status_code == 401, r1.status_code

        r2 = requests.get(self.agent_config_url,
                          params={"service.name": "foo"},
                          headers={
                              "Content-Type": "application/x-ndjson",
                              "Authorization": "Bearer " + self.config_overrides["secret_token"],
                          })
        assert r2.status_code == 503, r2.status_code

        config_request_logs = list(self.logged_requests(url="/config/v1/agents"))
        assert len(config_request_logs) == 2, config_request_logs
        self.assertDictContainsSubset({
            "level": "error",
            "message": "error handling request",
            "error": {"error": "invalid token"},
            "response_code": 401,
        }, config_request_logs[0])
        self.assertDictContainsSubset({
            "level": "error",
            "message": "error handling request",
            "response_code": 503,
        }, config_request_logs[1])


class AgentConfigurationKibanaDisabledIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "kibana_enabled": "false",
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_log_kill_switch_active(self):
        r = requests.get(self.agent_config_url,
                         headers={
                             "Content-Type": "application/x-ndjson",
                         })
        assert r.status_code == 403, r.status_code
        config_request_logs = list(self.logged_requests(url="/config/v1/agents"))
        self.assertDictContainsSubset({
            "level": "error",
            "message": "error handling request",
            "error": {"error": "forbidden request: endpoint is disabled"},
            "response_code": 403,
        }, config_request_logs[0])
