import unittest

from apmserver import ElasticTest, SubCommandTest, get_elasticsearch_url
from beat.beat import INTEGRATION_TESTS
from elasticsearch import Elasticsearch, NotFoundError
from elasticsearch.client import IngestClient


class SetupPipelinesDefaultTest(SubCommandTest):
    pipeline_name = "apm_user_agent"

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
                             "--pipelines"]
        }

    def setUp(self):
        # TODO (gr): consolidate with ElasticTest
        self.es = Elasticsearch([get_elasticsearch_url()])
        self.es.ingest.delete_pipeline(id="*")
        super(SetupPipelinesDefaultTest, self).setUp()

    def assert_pipeline_presence(self, should_exist=False):
        try:
            self.es.ingest.get_pipeline(self.pipeline_name)
            present = True
        except NotFoundError:
            present = False

        assert should_exist == present, "expected {}pipelines".format("" if should_exist else "no ")

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_setup_pipelines(self):
        self.assert_pipeline_presence(True)
        assert self.log_contains("Pipeline successfully registered: apm_user_agent")
        assert self.log_contains("Registered Ingest Pipelines successfully.")


class SetupPipelinesDisabledTest(SetupPipelinesDefaultTest):
    def config(self):
        cfg = super(SetupPipelinesDisabledTest, self).config()
        cfg.update({
            "register_pipeline_enabled": "false",
        })
        return cfg

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_setup_pipelines(self):
        self.assert_pipeline_presence(False)
        assert self.log_contains("No pipeline callback registered")


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
        assert self.log_contains("Index setup finished")
        # by default overwrite is set to true when `setup` cmd is run
        assert self.log_contains("Existing template will be overwritten, as overwrite is enabled.")
        self.assertNotRegexpMatches(self.get_log(), "ILM")
