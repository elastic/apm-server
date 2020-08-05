import json
import os
import random
import requests

from apmserver import BaseTest, integration_test
from helper import wait_until


class APIKeyHelper(object):
    # APIKeyHelper contains functions related to creating and invalidating API Keys and
    # waiting until the actions are completed.
    def __init__(self, es_url):
        # api_key related urls for configured user (default: apm_server_user)
        self.api_key_url = "{}/_security/api_key".format(es_url)
        self.privileges_url = "{}/_security/privilege".format(es_url)

    def wait_until_invalidated(self, name=None, id=None):
        if not name and not id:
            raise Exception("Either name or id must be given")

        def invalidated():
            keys = self.fetch_by_name(name) if name else self.fetch_by_id(id)
            for entry in keys:
                if not entry["invalidated"]:
                    return False
            return True
        wait_until(lambda: invalidated(), name="api keys invalidated")

    def wait_until_created(self, id):
        wait_until(lambda: len(self.fetch_by_id(id)) == 1, name="create api key")

    def fetch_by_name(self, name):
        resp = requests.get("{}?name={}".format(self.api_key_url, name))
        assert resp.status_code == 200
        assert "api_keys" in resp.json(), resp.json()
        return resp.json()["api_keys"]

    def fetch_by_id(self, id):
        resp = requests.get("{}?id={}".format(self.api_key_url, id))
        assert resp.status_code == 200, resp.status_code
        assert "api_keys" in resp.json(), resp.json()
        return resp.json()["api_keys"]

    def create(self, payload):
        resp = requests.post(self.api_key_url,
                             data=payload,
                             headers={'content-type': 'application/json'})
        assert resp.status_code == 200, resp.status_code
        self.wait_until_created(resp.json()["id"])
        return resp.json()

    def invalidate(self, name):
        resp = requests.delete(self.api_key_url,
                               data=json.dumps({'name': name}),
                               headers={'content-type': 'application/json'})
        self.wait_until_invalidated(name=name)
        return resp.json()


class APIKeyCommandBaseTest(BaseTest):
    apikey_name = "apm_integration_key"

    def config(self):
        return {
            "elasticsearch_host": self.es_url,
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def setUp(self):
        super(APIKeyCommandBaseTest, self).setUp()
        self.user = os.getenv("ES_USER", "apm_server_user")
        password = os.getenv("ES_PASS", "changeme")
        self.es_url = self.get_elasticsearch_url(self.user, password)
        self.kibana_url = self.get_kibana_url()
        # apikey_helper contains helper functions for base actions related to creating and invalidating api keys
        self.apikey_helper = APIKeyHelper(self.es_url)
        self.render_config_template(**self.config())

    def subcommand_output(self, *args, **kwargs):
        log = self.subcommand(*args, **kwargs)
        # command and go test output is combined in log, pull out the command output
        command_output = self._trim_golog(log)
        return json.loads(command_output)

    def subcommand(self, *args, **kwargs):
        logfile = self.beat_name + "-" + str(random.randint(0, 99999)) + "-" + args[0] + ".log"
        subcmd = ["apikey"]
        subcmd.extend(args)
        subcmd.append("--json")
        exit_code = self.run_beat(logging_args=[], extra_args=subcmd, output=logfile)
        log = self.get_log(logfile)
        assert exit_code == kwargs.get('exit_code', 0), log
        return log

    @staticmethod
    def _trim_golog(log):
        # If the command fails it will exit before printing coverage,
        # hence why this is conditional.
        pos = log.rfind("\nPASS\n")
        if pos >= 0:
            for trimmed in log[pos+1:].strip().splitlines():
                assert trimmed.split(None, 1)[0] in ("PASS", "coverage:"), trimmed
            log = log[:pos]
        return log

    def create(self, *args):
        apikey = self.subcommand_output("create", "--name", self.apikey_name, *args)
        self.apikey_helper.wait_until_created(apikey.get("id"))
        return apikey

    def invalidate_by_id(self, id):
        invalidated = self.subcommand_output("invalidate", "--id={}".format(id))
        self.apikey_helper.wait_until_invalidated(id=id)
        return invalidated

    def invalidate_by_name(self, name):
        invalidated = self.subcommand_output("invalidate", "--name", name)
        self.apikey_helper.wait_until_invalidated(name=name)
        return invalidated


@integration_test
class APIKeyCommandTest(APIKeyCommandBaseTest):
    """
    Tests the apikey subcommand.
    """

    def setUp(self):
        super(APIKeyCommandTest, self).setUp()
        invalidated = self.invalidate_by_name(self.apikey_name)
        assert invalidated.get("error_count") == 0

    def test_create_with_settings_override(self):
        apikey = self.create(
            "-E", "output.elasticsearch.enabled=false",
            "-E", "apm-server.api_key.elasticsearch.hosts=[{}]".format(self.es_url)
        )
        assert apikey.get("credentials") is not None, apikey

    def test_info_by_id(self):
        self.create()
        apikey = self.create()
        info = self.subcommand_output("info",  "--id={}".format(apikey["id"]))
        assert len(info.get("api_keys")) == 1, info
        assert info["api_keys"][0].get("username") == self.user, info
        assert info["api_keys"][0].get("id") == apikey["id"], info
        assert info["api_keys"][0].get("name") == apikey["name"], info
        assert info["api_keys"][0].get("invalidated") is False, info

    def test_info_by_name(self):
        apikey = self.create()
        invalidated = self.invalidate_by_id(apikey["id"])
        assert invalidated.get("error_count") == 0
        self.create()
        self.create()

        info = self.subcommand_output("info", "--name", self.apikey_name)
        # can't test exact number because these tests have side effects
        assert len(info.get("api_keys")) > 2, info

        info = self.subcommand_output("info", "--name", self.apikey_name, "--valid-only")
        assert len(info.get("api_keys")) == 2, info

    def test_verify_all(self):
        apikey = self.create()
        result = self.subcommand_output("verify", "--credentials={}".format(apikey["credentials"]))
        assert result == {'event:write': True, 'config_agent:read': True, 'sourcemap:write': True}, result

        for privilege in ["ingest", "sourcemap", "agent-config"]:
            result = self.subcommand_output(
                "verify", "--credentials={}".format(apikey["credentials"]), "--" + privilege)
            assert len(result) == 1, result
            assert list(result.values())[0] is True

    def test_verify_each(self):
        apikey = self.create("--ingest")
        result = self.subcommand_output("verify", "--credentials={}".format(apikey["credentials"]))
        assert result == {'event:write': True, 'config_agent:read': False, 'sourcemap:write': False}, result

        apikey = self.create("--sourcemap")
        result = self.subcommand_output("verify", "--credentials={}".format(apikey["credentials"]))
        assert result == {'event:write': False, 'config_agent:read': False, 'sourcemap:write': True}, result

        apikey = self.create("--agent-config")
        result = self.subcommand_output("verify", "--credentials={}".format(apikey["credentials"]))
        assert result == {'event:write': False, 'config_agent:read': True, 'sourcemap:write': False}, result


@integration_test
class APIKeyCommandBadUserTest(APIKeyCommandBaseTest):

    def config(self):
        return {
            "elasticsearch_host": self.get_elasticsearch_url(user="heartbeat_user", password="changeme"),
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def test_create_bad_user(self):
        """heartbeat_user doesn't have required cluster privileges, so it can't create keys"""
        result = self.subcommand_output("create", "--name", self.apikey_name, exit_code=1)
        assert result.get("error") is not None


@integration_test
class APIKeyCommandBadUser2Test(APIKeyCommandBaseTest):

    def config(self):
        return {
            "elasticsearch_host": self.get_elasticsearch_url(user="beats_user", password="changeme"),
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def test_create_bad_user(self):
        """beats_user does have required cluster privileges, but not APM application privileges,
        so it can't create keys
        """
        result = self.subcommand_output("create", "--name", self.apikey_name, exit_code=1)
        assert result.get("error") is not None, result
        assert "beats_user is missing the following requested privilege(s):" in result.get("error"), result
