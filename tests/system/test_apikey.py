from apmserver import BaseTest, integration_test
from elasticsearch import Elasticsearch
import inspect
import json
import random


class APIKeyBaseTest(BaseTest):
    api_key_name = "apm_integration_key"

    def config(self):
        return {
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def setUp(self):
        super(APIKeyBaseTest, self).setUp()
        self.es = Elasticsearch([self.get_elasticsearch_url()])
        self.kibana_url = self.get_kibana_url()
        self.render_config_template(**self.config())

    def subcommand_output(self, *args):
        log = self.subcommand(*args)
        # command and go test output is combined in log, pull out the command output
        command_output = self._trim_golog(log)
        return json.loads(command_output)

    def subcommand(self, *args):
        caller = inspect.getouterframes(inspect.currentframe())[1][3]
        logfile = self.beat_name + "-" + caller + str(random.randint(0, 9999)) + "-" + args[0] + ".log"
        subcmd = ["apikey"]
        subcmd.extend(args)
        subcmd.append("--json")
        self.run_beat(logging_args=[], extra_args=subcmd, output=logfile)
        return self.get_log(logfile)

    @staticmethod
    def _trim_golog(log):
        pos = -1
        for _ in range(2):
            pos = log[:pos].rfind("\n")
        command_output = log[:pos]
        for trimmed in log[pos:].strip().splitlines():
            assert trimmed.split(None, 1)[0] in ("PASS", "coverage:"), trimmed
        return command_output

    def create(self, *args):
        return self.subcommand_output("create", "--name", self.api_key_name, *args)


@integration_test
class APIKeyTest(APIKeyBaseTest):
    """
    Tests the apikey subcommand.
    """

    def tearDown(self):
        super(APIKeyBaseTest, self).tearDown()
        invalidated = self.subcommand_output("invalidate", "--name", self.api_key_name)
        assert invalidated["error_count"] == 0

    def test_create(self):
        apikey = self.create()

        assert apikey["name"] == self.api_key_name, apikey

        for privilege in ["sourcemap", "agentConfig", "event"]:
            apikey["created_privileges"]["apm"][privilege]["created"] = True, apikey

        for attr in ["id", "api_key", "credentials"]:
            assert apikey[attr] != "", apikey

    def test_create_with_settings_override(self):
        apikey = self.create(
            "-E", "output.elasticsearch.enabled=false",
            "-E", "apm-server.api_key.elasticsearch.hosts=[{}]".format(self.get_elasticsearch_url())
        )
        assert apikey["credentials"] is not None, apikey

    def test_create_with_expiration(self):
        apikey = self.create("--expiration", "1d")
        assert apikey["expiration"] is not None, apikey

    def test_invalidate_by_id(self):
        apikey = self.create()
        invalidated = self.subcommand_output("invalidate", "--id", apikey["id"])
        assert invalidated["invalidated_api_keys"] == [apikey["id"]], invalidated
        assert invalidated["error_count"] == 0, invalidated

    def test_invalidate_by_name(self):
        self.create()
        self.create()
        invalidated = self.subcommand_output("invalidate", "--name", self.api_key_name)
        assert len(invalidated["invalidated_api_keys"]) == 2, invalidated
        assert invalidated["error_count"] == 0, invalidated

    def test_info_by_id(self):
        self.create()
        apikey = self.create()
        info = self.subcommand_output("info", "--id", apikey["id"])
        assert len(info["api_keys"]) == 1, info
        assert info["api_keys"][0]["username"] == "admin", info
        assert info["api_keys"][0]["id"] == apikey["id"], info
        assert info["api_keys"][0]["name"] == apikey["name"], info
        assert info["api_keys"][0]["invalidated"] is False, info

    def test_info_by_name(self):
        apikey = self.create()
        invalidated = self.subcommand_output("invalidate", "--id", apikey["id"])
        assert invalidated["error_count"] == 0
        self.create()
        self.create()

        info = self.subcommand_output("info", "--name", self.api_key_name)
        # can't test exact number because these tests have side effects
        assert len(info["api_keys"]) > 2, info

        info = self.subcommand_output("info", "--name", self.api_key_name, "--valid-only")
        assert len(info["api_keys"]) == 2, info

    def test_verify_all(self):
        apikey = self.create()
        result = self.subcommand_output("verify", "--credentials", apikey["credentials"])
        assert result == {'event:write': True, 'config_agent:read': True, 'sourcemap:write': True}, result

        for privilege in ["ingest", "sourcemap", "agent-config"]:
            result = self.subcommand_output("verify", "--credentials", apikey["credentials"], "--" + privilege)
            assert len(result) == 1, result
            assert result.values()[0] is True

    def test_verify_each(self):
        apikey = self.create("--ingest")
        result = self.subcommand_output("verify", "--credentials", apikey["credentials"])
        assert result == {'event:write': True, 'config_agent:read': False, 'sourcemap:write': False}, result

        apikey = self.create("--sourcemap")
        result = self.subcommand_output("verify", "--credentials", apikey["credentials"])
        assert result == {'event:write': False, 'config_agent:read': False, 'sourcemap:write': True}, result

        apikey = self.create("--agent-config")
        result = self.subcommand_output("verify", "--credentials", apikey["credentials"])
        assert result == {'event:write': False, 'config_agent:read': True, 'sourcemap:write': False}, result


@integration_test
class APIKeyBadUserTest(APIKeyBaseTest):

    def config(self):
        return {
            "elasticsearch_host": self.get_elasticsearch_url(user="apm_server_user", password="changeme"),
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def test_create_bad_user(self):
        out = self.subcommand("create", "--name", self.api_key_name)
        result = json.loads(out)
        assert result["status"] == 401, result
        assert result["error"] is not None
