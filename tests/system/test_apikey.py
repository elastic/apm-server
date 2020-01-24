import json
import os
import random

from apmserver import BaseTest, integration_test


class APIKeyBaseTest(BaseTest):
    apikey_name = "apm_integration_key"

    def config(self):
        return {
            "elasticsearch_host": self.es_url,
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def setUp(self):
        super(APIKeyBaseTest, self).setUp()
        self.user = os.getenv("ES_USER", "apm_server_user")
        password = os.getenv("ES_PASS", "changeme")
        self.es_url = self.get_elasticsearch_url(self.user, password)
        self.kibana_url = self.get_kibana_url()
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
        return self.subcommand_output("create", "--name", self.apikey_name, *args)


@integration_test
class APIKeyTest(APIKeyBaseTest):
    """
    Tests the apikey subcommand.
    """

    def setUp(self):
        super(APIKeyTest, self).setUp()
        invalidated = self.subcommand_output("invalidate", "--name", self.apikey_name)
        assert invalidated.get("error_count") == 0

    def test_create(self):
        apikey = self.create()

        assert apikey.get("name") == self.apikey_name, apikey

        for attr in ["id", "api_key", "credentials"]:
            assert apikey.get(attr) != "", apikey

    def test_create_with_settings_override(self):
        apikey = self.create(
            "-E", "output.elasticsearch.enabled=false",
            "-E", "apm-server.api_key.elasticsearch.hosts=[{}]".format(self.es_url)
        )
        assert apikey.get("credentials") is not None, apikey

    def test_create_with_expiration(self):
        apikey = self.create("--expiration", "1d")
        assert apikey.get("expiration") is not None, apikey

    def test_invalidate_by_id(self):
        apikey = self.create()
        invalidated = self.subcommand_output("invalidate", "--id", apikey["id"])
        assert invalidated.get("invalidated_api_keys") == [apikey["id"]], invalidated
        assert invalidated.get("error_count") == 0, invalidated

    def test_invalidate_by_name(self):
        self.create()
        self.create()
        invalidated = self.subcommand_output("invalidate", "--name", self.apikey_name)
        assert len(invalidated.get("invalidated_api_keys")) == 2, invalidated
        assert invalidated.get("error_count") == 0, invalidated

    def test_info_by_id(self):
        self.create()
        apikey = self.create()
        info = self.subcommand_output("info", "--id", apikey["id"])
        assert len(info.get("api_keys")) == 1, info
        assert info["api_keys"][0].get("username") == self.user, info
        assert info["api_keys"][0].get("id") == apikey["id"], info
        assert info["api_keys"][0].get("name") == apikey["name"], info
        assert info["api_keys"][0].get("invalidated") is False, info

    def test_info_by_name(self):
        apikey = self.create()
        invalidated = self.subcommand_output("invalidate", "--id", apikey["id"])
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
            "elasticsearch_host": self.get_elasticsearch_url(user="heartbeat_user", password="changeme"),
            "file_enabled": "false",
            "kibana_enabled": "false",
        }

    def test_create_bad_user(self):
        """heartbeat_user doesn't have required cluster privileges, so it can't create keys"""
        result = self.subcommand_output("create", "--name", self.apikey_name, exit_code=1)
        assert result.get("error") is not None


@integration_test
class APIKeyBadUser2Test(APIKeyBaseTest):

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
