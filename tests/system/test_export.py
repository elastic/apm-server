import yaml

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


class ExportTemplateDefaultTest(SubCommandTest):
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
        Test export default template
        """
        template = yaml.load(self.command_output)
        settings = template["apm-8.0.0"]["settings"]
        del(settings["index"]["query"])
        self.assertDictEqual(
            {
                "index": {
                    "codec": "best_compression",
                    "mapping": {
                        "total_fields": {"limit": 2000}
                    },
                    "number_of_routing_shards": 30,
                    "number_of_shards": 1,
                    "refresh_interval": "5s",
                },
            }, settings)


class ExportTemplateTest(SubCommandTest):
    """
    Test export template subcommand.
    """

    def start_args(self):
        return {
            "extra_args": ["export", "template",
                           "-E", "setup.template.settings.index.mapping.total_fields.limit=5",
                           ],
            "logging_args": None,
        }

    def test_export_template(self):
        """
        Test export customized template
        """
        template = yaml.load(self.command_output)
        settings = template["apm-8.0.0"]["settings"]
        del(settings["index"]["query"])
        self.assertDictEqual(
            {
                "index": {
                    "codec": "best_compression",
                    "mapping": {
                        "total_fields": {"limit": 5}
                    },
                    "number_of_routing_shards": 30,
                    "number_of_shards": 1,
                    "refresh_interval": "5s",
                },
            }, settings)
