import time
import unittest
from urlparse import urljoin
import uuid

import requests

from apmserver import ElasticTest
from beat.beat import INTEGRATION_TESTS


class AgentConfigurationIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "kibana_enabled": "true",
        "acm_cache_expiration": "1s",
    }

    def config(self):
        cfg = super(ElasticTest, self).config()
        cfg.update({
            "kibana_host": self.get_kibana_url(),
        })
        cfg.update(self.config_overrides)
        return cfg

    def create_service_config(self, settings, name, env=None, _id="new"):
        data = {
            "service": {"name": name},
            "settings": settings
        }
        if env is not None:
            data["service"]["environment"] = env
        meth = requests.post if _id == "new" else requests.put
        return meth(
            urljoin(self.kibana_url, "/api/apm/settings/agent-configuration/{}".format(_id)),
            headers={
                "Accept": "*/*",
                "Content-Type": "application/json",
                "kbn-xsrf": "1",
            },
            json=data,
        )

    def update_service_config(self, settings, _id, name, env=None):
        return self.create_service_config(settings, name, env, _id=_id)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_config_requests(self):
        service_name = uuid.uuid4().hex
        service_env = "production"
        bad_service_env = "notreal"

        expect_log = []

        # missing service.name
        r1 = requests.get(self.agent_config_url,
                          headers={"Content-Type": "application/x-ndjson"},
                          )
        assert r1.status_code == 400, r1.status_code
        expect_log.append({
            "level": "error",
            "message": "invalid query",
            "error": "service.name is required",
            "response_code": 400,
        })

        # no configuration for service
        r2 = requests.get(self.agent_config_url,
                          params={"service.name": service_name + "_cache_bust"},
                          headers={"Content-Type": "application/x-ndjson"},
                          )
        assert r2.status_code == 200, r2.status_code
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })
        self.assertDictEqual({}, r2.json())

        create_config_rsp = self.create_service_config({"transaction_sample_rate": 0.05}, service_name)
        create_config_rsp.raise_for_status()
        assert create_config_rsp.status_code == 200, create_config_rsp.status_code
        create_config_result = create_config_rsp.json()
        assert create_config_result["result"] == "created"

        # yes configuration for service
        r3 = requests.get(self.agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/x-ndjson"})
        assert r3.status_code == 200, r3.status_code
        # TODO (gr): validate Cache-Control header - https://github.com/elastic/apm-server/issues/2438
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })
        self.assertDictEqual({"transaction_sample_rate": "0.05"}, r3.json())

        # not modified on re-request
        r3_again = requests.get(self.agent_config_url,
                                params={"service.name": service_name},
                                headers={
                                    "Content-Type": "application/x-ndjson",
                                    "If-None-Match": r3.headers["Etag"],
                                })
        assert r3_again.status_code == 304, r3_again.status_code
        expect_log.append({
            "level": "info",
            "message": "not modified",
            "response_code": 304,
        })

        # no configuration for service+environment
        r4 = requests.get(self.agent_config_url,
                          params={
                              "service.name": service_name,
                              "service.environment": bad_service_env,
                          },
                          headers={"Content-Type": "application/x-ndjson"})
        assert r4.status_code == 200, r4.status_code
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })
        self.assertDictEqual({}, r4.json())

        create_config_with_env_rsp = self.create_service_config(
            {"transaction_sample_rate": 0.15}, service_name, env=service_env)
        assert create_config_with_env_rsp.status_code == 200, create_config_with_env_rsp.status_code
        create_config_with_env_result = create_config_with_env_rsp.json()
        assert create_config_with_env_result["result"] == "created"
        create_config_with_env_id = create_config_with_env_result["_id"]

        # yes configuration for service+environment
        r5 = requests.get(self.agent_config_url,
                          params={
                              "service.name": service_name,
                              "service.environment": service_env,
                          },
                          headers={"Content-Type": "application/x-ndjson"})
        assert r5.status_code == 200, r5.status_code
        self.assertDictEqual({"transaction_sample_rate": "0.15"}, r5.json())
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })

        # not modified on re-request
        r5_again = requests.get(self.agent_config_url,
                                params={
                                    "service.name": service_name,
                                    "service.environment": service_env,
                                },
                                headers={
                                    "Content-Type": "application/x-ndjson",
                                    "If-None-Match": r5.headers["Etag"],
                                })
        assert r5_again.status_code == 304, r5_again.status_code
        expect_log.append({
            "level": "info",
            "message": "not modified",
            "response_code": 304,
        })

        updated_config_with_env_rsp = self.update_service_config(
            {"transaction_sample_rate": 0.99}, create_config_with_env_id, service_name, env=service_env)
        assert updated_config_with_env_rsp.status_code == 200, updated_config_with_env_rsp.status_code
        # TODO (gr): remove when cache can be disabled via config
        # wait for cache to purge
        time.sleep(1.1)  # sleep much more than acm_cache_expiration to reduce flakiness

        r5_post_update = requests.get(self.agent_config_url,
                                      params={
                                          "service.name": service_name,
                                          "service.environment": service_env,
                                      },
                                      headers={
                                          "Content-Type": "application/x-ndjson",
                                          "If-None-Match": r5.headers["Etag"],
                                      })
        assert r5_post_update.status_code == 200, r5_post_update.status_code
        self.assertDictEqual({"transaction_sample_rate": "0.99"}, r5_post_update.json())
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })

        config_request_logs = list(self.logged_requests(url="/config/v1/agents"))
        assert len(config_request_logs) == len(expect_log)
        for want, got in zip(expect_log, config_request_logs):
            self.assertDictContainsSubset(want, got)


class AgentConfigurationKibanaDownIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "secret_token": "supersecret",
        "kibana_enabled": "true",
        "kibana_host": "unreachablehost"
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_config_requests(self):
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
            "message": "invalid token",
            "error": "invalid token",
            "response_code": 401,
        }, config_request_logs[0])
        self.assertDictContainsSubset({
            "level": "error",
            "message": "unable to retrieve connection to Kibana",
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
            "message": "forbidden request",
            "error": "forbidden request: endpoint is disabled",
            "response_code": 403,
        }, config_request_logs[0])
