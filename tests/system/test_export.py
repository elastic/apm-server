import json
import os
import yaml

from apmserver import ServerSetUpBaseTest


class SubCommandTest(ServerSetUpBaseTest):
    def wait_until_started(self):
        self.apmserver_proc.check_wait()

        # command and go test output is combined in log, pull out the command output
        log = self.get_log()
        pos = -1
        for _ in range(2):
            pos = log[:pos].rfind(os.linesep)
        self.command_output = log[:pos]
        for trimmed in log[pos:].strip().splitlines():
            # ensure only skipping expected lines
            assert trimmed.split(None, 1)[0] in ("PASS", "coverage:"), trimmed


class ExportConfigTest(SubCommandTest):
    """
    Test export config subcommand.
    """

    def start_args(self):
        return {
            "extra_args": ["export", "config"],
            "logging_args": None,
        }

    def test_export_config(self):
        """
        Test export config works
        """
        config = yaml.load(self.command_output)
        total_fields_limit = config['setup']['template']['settings']['index']['mapping']['total_fields']['limit']
        assert total_fields_limit == 2000, total_fields_limit


class ExportTemplateTest(SubCommandTest):
    """
    Test export template subcommand.
    """

    def start_args(self):
        return {
            "extra_args": ["export", "template"],
            "logging_args": None,
        }

    def test_export_template(self):
        """
        Test export template works
        """
        template = json.loads(self.command_output)
        total_fields_limit = template['settings']['index']['mapping']['total_fields']['limit']
        assert total_fields_limit == 2000, total_fields_limit
