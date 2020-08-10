import time
from urllib.parse import urljoin
import uuid
import requests

from apmserver import ElasticTest, integration_test, is_subset


class AgentConfigurationTest(ElasticTest):

    def config(self):
        cfg = super(AgentConfigurationTest, self).config()
        cfg.update({
            "kibana_host": self.get_kibana_url(),
            "logging_json": "true",
            "kibana_enabled": "true",
            "acm_cache_expiration": "1s"
        })
        cfg.update(self.config_overrides)
        return cfg

    def create_service_config(self, settings, name, agent="python", env=None):
        return self.kibana.create_agent_config(name, settings, agent=agent, env=env)

    def update_service_config(self, settings, name, env=None):
        res = self.kibana.create_or_update_agent_config(name, settings, env=env)
        assert res.json()["result"] == "updated"


@integration_test
class AgentConfigurationIntegrationTest(AgentConfigurationTest):
    def test_config_requests(self):
        service_name = uuid.uuid4().hex
        service_env = "production"
        bad_service_env = "notreal"

        expect_log = []

        # missing service.name
        r1 = requests.get(self.agent_config_url,
                          headers={"Content-Type": "application/json"},
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
                          headers={"Content-Type": "application/json"},
                          )
        assert r2.status_code == 200, r2.status_code
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })
        self.assertEqual({}, r2.json())

        self.create_service_config({"transaction_sample_rate": "0.05"}, service_name)

        # yes configuration for service
        r3 = requests.get(self.agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})
        assert r3.status_code == 200, r3.status_code
        # TODO (gr): validate Cache-Control header - https://github.com/elastic/apm-server/issues/2438
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })
        self.assertEqual({"transaction_sample_rate": "0.05"}, r3.json())

        # not modified on re-request
        r3_again = requests.get(self.agent_config_url,
                                params={"service.name": service_name},
                                headers={
                                    "Content-Type": "application/json",
                                    "If-None-Match": r3.headers["Etag"],
                                })
        assert r3_again.status_code == 304, r3_again.status_code
        expect_log.append({
            "level": "info",
            "message": "not modified",
            "response_code": 304,
        })

        self.create_service_config(
            {"transaction_sample_rate": "0.15"}, service_name, env=service_env)

        # yes configuration for service+environment
        r4 = requests.get(self.agent_config_url,
                          params={
                              "service.name": service_name,
                              "service.environment": service_env,
                          },
                          headers={"Content-Type": "application/json"})
        assert r4.status_code == 200, r4.status_code
        self.assertEqual({"transaction_sample_rate": "0.15"}, r4.json())
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })

        # not modified on re-request
        r4_again = requests.get(self.agent_config_url,
                                params={
                                    "service.name": service_name,
                                    "service.environment": service_env,
                                },
                                headers={
                                    "Content-Type": "application/json",
                                    "If-None-Match": r4.headers["Etag"],
                                })
        assert r4_again.status_code == 304, r4_again.status_code
        expect_log.append({
            "level": "info",
            "message": "not modified",
            "response_code": 304,
        })

        self.update_service_config(
            {"transaction_sample_rate": "0.99"}, service_name, env=service_env)

        # TODO (gr): remove when cache can be disabled via config
        # wait for cache to purge
        time.sleep(1.1)  # sleep much more than acm_cache_expiration to reduce flakiness

        r4_post_update = requests.get(self.agent_config_url,
                                      params={
                                          "service.name": service_name,
                                          "service.environment": service_env,
                                      },
                                      headers={
                                          "Content-Type": "application/json",
                                          "If-None-Match": r4.headers["Etag"],
                                      })
        assert r4_post_update.status_code == 200, r4_post_update.status_code
        self.assertEqual({"transaction_sample_rate": "0.99"}, r4_post_update.json())
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })

        # configuration for service+environment (all includes non existing)
        r5 = requests.get(self.agent_config_url,
                          params={
                              "service.name": service_name,
                              "service.environment": bad_service_env,
                          },
                          headers={"Content-Type": "application/json"})
        assert r5.status_code == 200, r5.status_code
        expect_log.append({
            "level": "info",
            "message": "request ok",
            "response_code": 200,
        })
        self.assertEqual({"transaction_sample_rate": "0.05"}, r5.json())

        config_request_logs = list(self.logged_requests(url="/config/v1/agents"))
        assert len(config_request_logs) == len(expect_log)
        for want, got in zip(expect_log, config_request_logs):
            assert is_subset(want, got)

    def test_rum_disabled(self):
        r = requests.get(self.rum_agent_config_url,
                         params={
                             "service.name": "rum-service",
                         },
                         headers={"Content-Type": "application/json"}
                         )
        assert r.status_code == 403
        assert "RUM endpoint is disabled" in r.json().get('error'), r.json()


@integration_test
class AgentConfigurationKibanaDownIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "secret_token": "supersecret",
        "kibana_enabled": "true",
        "kibana_host": "unreachablehost"
    }

    def test_config_requests(self):
        r1 = requests.get(self.agent_config_url,
                          headers={
                              "Content-Type": "application/json",
                          })
        assert r1.status_code == 401, r1.status_code

        r2 = requests.get(self.agent_config_url,
                          params={"service.name": "foo"},
                          headers={
                              "Content-Type": "application/json",
                              "Authorization": "Bearer " + self.config_overrides["secret_token"],
                          })
        assert r2.status_code == 503, r2.status_code

        config_request_logs = list(self.logged_requests(url="/config/v1/agents"))
        assert len(config_request_logs) == 2, config_request_logs
        assert is_subset({
            "level": "error",
            "message": "unauthorized",
            "error": "unauthorized",
            "response_code": 401,
        }, config_request_logs[0])
        assert is_subset({
            "level": "error",
            "message": "unable to retrieve connection to Kibana",
            "response_code": 503,
        }, config_request_logs[1])


@integration_test
class AgentConfigurationKibanaDisabledIntegrationTest(ElasticTest):
    config_overrides = {
        "logging_json": "true",
        "kibana_enabled": "false",
    }

    def test_log_kill_switch_active(self):
        r = requests.get(self.agent_config_url,
                         headers={
                             "Content-Type": "application/json",
                         })
        assert r.status_code == 403, r.status_code
        config_request_logs = list(self.logged_requests(url="/config/v1/agents"))
        assert is_subset({
            "level": "error",
            "message": "forbidden request",
            "response_code": 403,
        }, config_request_logs[0])


@integration_test
class RumAgentConfigurationIntegrationTest(AgentConfigurationTest):
    config_overrides = {
        "enable_rum": "true",
    }

    def test_rum(self):
        service_name = "rum-service"
        self.create_service_config({"transaction_sample_rate": "0.2"}, service_name, agent="rum-js")

        r1 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r1.status_code == 200
        assert r1.json() == {'transaction_sample_rate': '0.2'}
        etag = r1.headers["Etag"].replace('"', '')  # RUM will send it without double quotes

        r2 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name, "ifnonematch": etag},
                          headers={"Content-Type": "application/json"})
        assert r2.status_code == 304

    def test_rum_current_name(self):
        service_name = "rum-service"
        self.create_service_config({"transaction_sample_rate": "0.2"}, service_name, agent="js-base")

        r1 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r1.status_code == 200
        assert r1.json() == {'transaction_sample_rate': '0.2'}
        etag = r1.headers["Etag"].replace('"', '')  # RUM will send it without double quotes

        r2 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name, "ifnonematch": etag},
                          headers={"Content-Type": "application/json"})
        assert r2.status_code == 304

    def test_backend_after_rum(self):
        service_name = "backend-service"
        self.create_service_config({"transaction_sample_rate": "0.3"}, service_name)

        r1 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r1.status_code == 200, r1.status_code
        assert r1.json() == {}, r1.json()

        r2 = requests.get(self.agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r2.status_code == 200, r2.status_code
        assert r2.json() == {"transaction_sample_rate": "0.3"}, r2.json()

    def test_rum_after_backend(self):
        service_name = "backend-service"
        self.create_service_config({"transaction_sample_rate": "0.3"}, service_name)

        r1 = requests.get(self.agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r1.status_code == 200, r1.status_code
        assert r1.json() == {"transaction_sample_rate": "0.3"}, r1.json()

        r2 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r2.status_code == 200, r2.status_code
        assert r2.json() == {}, r2.json()

    def test_all_agents(self):
        service_name = "any-service"
        self.create_service_config(
            {"transaction_sample_rate": "0.4", "capture_body": "all"}, service_name, agent="")

        r1 = requests.get(self.rum_agent_config_url,
                          params={"service.name": service_name},
                          headers={"Content-Type": "application/json"})

        assert r1.status_code == 200, r1.status_code
        # only return settings applicable to RUM
        assert r1.json() == {"transaction_sample_rate": "0.4"}, r1.json()
