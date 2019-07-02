import yaml
import os
import json
import shutil
from apmserver import SubCommandTest


class ExportConfigDefaultTest(SubCommandTest):
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
        Test export default config
        """
        config = yaml.load(self.command_output)
        # logging settings
        self.assertDictEqual(
            {"metrics": {"enabled": False}}, config["logging"]
        )

        # template settings
        self.assertDictEqual(
            {
                "template": {
                    "settings": {
                        "_source": {"enabled": True},
                        "index": {
                            "codec": "best_compression",
                            "mapping": {
                                "total_fields": {"limit": 2000}
                            },
                            "number_of_shards": 1,
                        },
                    },
                },
            }, config["setup"])


class ExportConfigTest(SubCommandTest):
    """
    Test export config subcommand.
    """

    def start_args(self):
        return {
            "extra_args": ["export", "config",
                           "-E", "logging.metrics.enabled=true",
                           "-E", "setup.template.settings.index.mapping.total_fields.limit=5",
                           ],
            "logging_args": None,
        }

    def test_export_config(self):
        """
        Test export customized config
        """
        config = yaml.load(self.command_output)
        # logging settings
        self.assertDictEqual(
            {"metrics": {"enabled": True}}, config["logging"]
        )

        # template settings
        self.assertDictEqual(
            {
                "template": {
                    "settings": {
                        "_source": {"enabled": True},
                        "index": {
                            "codec": "best_compression",
                            "mapping": {
                                "total_fields": {"limit": 5}
                            },
                            "number_of_shards": 1,
                        },
                    },
                },
            }, config["setup"])


class TestExportTemplate(SubCommandTest):
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
        super(TestExportTemplate, self).setUp()

    def tearDown(self):
        shutil.rmtree(self.dir)

    def test_export_template_to_file(self):
        """
        Test export general apm template to file
        """
        file = os.path.join(self.dir, "template", self.index_name + '.json')
        with open(file) as f:
            template = json.load(f)
        assert template['index_patterns'] == [self.index_name + '*']
        assert template['settings']['index']['mapping']['total_fields']['limit'] == 5
        assert len(template['mappings']) > 0
        assert template['order'] == 1

    def test_export_event_templates_to_file(self):
        """
        Test export default event templates without ILM
        """
        for e in ['error', 'span', 'transaction', 'metric']:
            name = "{}-{}".format(self.index_name, e)
            file = os.path.join(self.dir, "template", name + '.json')
            with open(file) as f:
                template = json.load(f)
            assert template['index_patterns'] == [name + '*']
            assert template['settings']['index'] == None
            assert 'mapping' not in template
            assert template['order'] == 2


class TestExportTemplateWithILM(SubCommandTest):
    """
    Test export template with ilm
    """

    def start_args(self):
        return {
            "extra_args": ["export", "template", "--dir", self.dir,
                           "-E", "apm-server.ilm.enabled=true",
                           "-E", "output.elasticsearch.enabled=true"],
        }

    def setUp(self):
        self.dir = os.path.abspath(os.path.join(self.beat_path, os.path.dirname(__file__), "test-export-template-ilm"))
        super(TestExportTemplateWithILM, self).setUp()

    def tearDown(self):
        shutil.rmtree(self.dir)

    def test_export_template_to_file(self):
        """
        Test export default ilm policy
        """
        file = os.path.join(self.dir, "template", self.index_name + '.json')
        with open(file) as f:
            template = json.load(f)
        assert template['index_patterns'] == [self.index_name + '*']
        assert len(template['mappings']) > 0
        assert template['order'] == 1

    def test_export_event_templates_to_file(self):
        """
        Test export default event templates
        """
        for e in ['error', 'span', 'transaction', 'metric']:
            name = "{}-{}".format(self.index_name, e)
            file = os.path.join(self.dir, "template", name + '.json')
            with open(file) as f:
                template = json.load(f)
            assert template['index_patterns'] == [name + '*']
            assert template['settings']['index'] is not None
            assert template['settings']['index']['lifecycle.name'] == name
            assert template['settings']['index']['lifecycle.rollover_alias'] == name
            assert 'mapping' not in template
            assert template['order'] == 2


class TestExportILMPolicy(SubCommandTest):
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
        super(TestExportILMPolicy, self).setUp()

    def tearDown(self):
        shutil.rmtree(self.dir)

    def test_export_ilm_policy_to_files(self):
        """
        Test export default ilm policy
        """
        for e in ['error', 'span', 'transaction', 'metric']:
            name = "{}-{}".format(self.index_name, e)
            file = os.path.join(self.dir, "policy", name + '.json')
            with open(file) as f:
                policy = json.load(f)
            assert "hot" in policy["policy"]["phases"]
            assert "warm" in policy["policy"]["phases"]
            assert "delete" not in policy["policy"]["phases"]


class TestExportILMPolicyILMDisabled(TestExportILMPolicy):
    """
    Test export ilm-policy independent of ILM enabled state
    """

    def start_args(self):
        return {
            "extra_args": ["export", "ilm-policy", "--dir", self.dir,
                           "-E", "apm-server.ilm.enabled=false"],
        }
