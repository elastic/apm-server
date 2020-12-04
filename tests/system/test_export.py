import yaml
import os
import json
import shutil
from apmserver import SubCommandTest, integration_test
from es_helper import index_name
from yaml import Loader


class ExportCommandTest(SubCommandTest):
    config_overrides = {"default_setup_template_settings": True}
    register_pipeline_disabled = True


@integration_test
class TestExportTemplate(ExportCommandTest):
    """
    Test export template
    """

    def start_args(self):
        return {
            "extra_args": ["export", "template", "--dir", self.dir,
                           "-E", "setup.template.settings.index.mapping.total_fields.limit=5",
                           "-E", "apm-server.ilm.enabled=false"],
        }

    def setUp(self):
        self.dir = os.path.abspath(os.path.join(self.beat_path, os.path.dirname(__file__), "test-export-template"))
        self.addCleanup(self.cleanup_exports)
        super(TestExportTemplate, self).setUp()

    def cleanup_exports(self):
        shutil.rmtree(self.dir)

    def test_export_template_to_file(self):
        """
        Test export general apm template to file
        """
        path = os.path.join(self.dir, "template", index_name + '.json')
        with open(path) as f:
            template = json.load(f)
        assert template['index_patterns'] == [index_name + '*']
        assert template['settings']['index']['mapping']['total_fields']['limit'] == 5
        assert len(template['mappings']) > 0
        assert template['order'] == 1


@integration_test
class TestExportILMPolicy(ExportCommandTest):
    """
    Test export ilm-policy
    """

    def start_args(self):
        return {
            "extra_args": ["export", "ilm-policy", "--dir", self.dir,
                           "-E", "apm-server.ilm.enabled=true"],
        }

    def setUp(self):
        self.dir = os.path.abspath(os.path.join(self.beat_path, os.path.dirname(__file__), "test-export-ilm"))
        self.addCleanup(self.cleanup_exports)
        super(TestExportILMPolicy, self).setUp()

    def cleanup_exports(self):
        shutil.rmtree(self.dir)

    def test_export_ilm_policy_to_files(self):
        """
        Test export default ilm policy
        """

        assert os.path.exists(self.dir)
        dir = os.path.join(self.dir, "policy")
        for subdir, dirs, files in os.walk(dir):
            assert len(files) == 1, files
            for file in files:
                with open(os.path.join(dir, file)) as f:
                    policy = json.load(f)
                assert "hot" in policy["policy"]["phases"]
                assert "warm" in policy["policy"]["phases"]
                assert "delete" not in policy["policy"]["phases"]


@integration_test
class TestExportILMPolicyILMDisabled(TestExportILMPolicy):
    """
    Test export ilm-policy independent of ILM enabled state
    """

    def start_args(self):
        return {
            "extra_args": ["export", "ilm-policy", "--dir", self.dir,
                           "-E", "apm-server.ilm.enabled=false"],
        }
