import unittest

from apmserver import SubCommandTest, get_elasticsearch_url
from beat.beat import INTEGRATION_TESTS
from elasticsearch import Elasticsearch


class SetupTemplateDefaultTest(SubCommandTest):
    """
    Test setup template subcommand with default option.
    """

    def config(self):
        cfg = super(SubCommandTest, self).config()
        cfg.update({
            "elasticsearch_host": get_elasticsearch_url(),
            "file_enabled": "false",
        })
        return cfg

    def start_args(self):
        return {
            "logging_args": ["-v", "-d", "*"],
            "extra_args":   ["-e",
                             "setup",
                             "-template"]
        }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_setup_default_template(self):
        """
        Test setup default template
        """

        es = Elasticsearch([get_elasticsearch_url()])
        assert es.indices.exists_template(name='apm-*')
        assert self.log_contains("Loaded index template")
        assert self.log_contains("Index setup complete")
        # by default overwrite is set to true when `setup` cmd is run
        assert self.log_contains("Existing template will be overwritten, as overwrite is enabled.")
        self.assertNotRegexpMatches(self.get_log(), "ILM")
