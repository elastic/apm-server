import unittest
from apmserver import ElasticTest, SubCommandTest, get_elasticsearch_url
from beat.beat import INTEGRATION_TESTS
from elasticsearch import Elasticsearch, NotFoundError


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



class PipelineRegisterTest(ElasticTest):
    config_overrides = {
        "register_pipeline_enabled": "true",
        "register_pipeline_overwrite": "true"
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_default_pipelines_registered(self):
        pipelines = [
            ("apm_user_agent", "Add user agent information for APM events"),
            ("apm_user_geo", "Add user geo information for APM events"),
            ("apm", "Default enrichment for APM events"),
        ]
        loaded_msg = "Pipeline successfully registered"
        self.wait_until(lambda: self.log_contains(loaded_msg), max_timeout=5)
        for pipeline_id, pipeline_desc in pipelines:
            pipeline = self.es.ingest.get_pipeline(id=pipeline_id)
            assert pipeline[pipeline_id]['description'] == pipeline_desc


class PipelineDefaultOverwriteTest(ElasticTest):
    config_overrides = {
        "register_pipeline_enabled": "true",
    }

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_pipeline_not_overwritten(self):
        loaded_msg = "Pipeline already registered"
        self.wait_until(lambda: self.log_contains(loaded_msg),
                        max_timeout=5)


class PipelineDisableTest(ElasticTest):
    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_pipeline_not_registered(self):
        loaded_msg = "No pipeline callback registered"
        self.wait_until(lambda: self.log_contains(loaded_msg),
                        max_timeout=5)
