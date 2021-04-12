#import json
from urllib.parse import urljoin

import requests


class Kibana(object):
    def __init__(self, url):
        self.url = url

    def create_agent_config(self, service, settings, agent=None, env=None):
        return self._upsert_agent_config(service, settings, agent=agent, env=env)

    def create_or_update_agent_config(self, service, settings, agent=None, env=None):
        return self._upsert_agent_config(service, settings, agent=agent, env=env, overwrite=True)

    def _upsert_agent_config(self, service, settings, agent=None, env=None, overwrite=False):
        data = {
            "service": {"name": service},
            "settings": settings,
        }
        if agent is not None:
            data["agent_name"] = agent
        if env is not None:
            data["service"]["environment"] = env

        resp = requests.put(
            urljoin(self.url, "/api/apm/settings/agent-configuration"),
            headers={
                "Accept": "*/*",
                "Content-Type": "application/json",
                "kbn-xsrf": "1",
            },
            params={"overwrite": "true" if overwrite else "false"},
            json=data,
        )
        assert resp.status_code == 200, resp.status_code
        resp.raise_for_status()
        return resp

    def delete_all_agent_config(self):
        configs = self.list_agent_config()
        for config in configs:
            service = config["service"]
            self.delete_agent_config(
                service["name"], env=service.get("environment", None))

    def list_agent_config(self):
        resp = requests.get(
            urljoin(self.url, "/api/apm/settings/agent-configuration"),
            headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
                "kbn-xsrf": "1",
            }
        )
        assert resp.status_code == 200, resp.status_code
        return resp.json()['configurations']

    def delete_agent_config(self, service, env=None):
        data = {
            "service": {"name": service},
        }
        if env is not None:
            data["service"]["environment"] = env

        resp = requests.delete(
            urljoin(self.url, "/api/apm/settings/agent-configuration"),
            headers={
                "Accept": "*/*",
                "Content-Type": "application/json",
                "kbn-xsrf": "1",
            },
            json=data,
        )
        assert resp.status_code == 200, resp.status_code
        return resp
