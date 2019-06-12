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

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_default_pipelines_applied(self):
        self.wait_until(lambda: self.log_contains("Registered Ingest Pipelines successfully"), max_timeout=5)
        self.wait_until(lambda: self.log_contains("Finished index management setup."), max_timeout=5)

        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 3)
        entries = self.es.search(index=self.index_transaction)['hits']['hits']
        for e in entries:
            src = e['_source']
            ua = src['user_agent']
            id = src['transaction']['id']
            if id != "4340a8e0df1906ecbfa9":
                # transaction with id '4340a8e0df1906ecbfa9' has user agent info set
                assert ua is None
                continue
            else:
                assert ua is not None
                assert ua["original"] == "Mozilla Chrome Edge, Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36"
                assert ua["name"] == "Chrome"
                assert ua["version"] == "51.0.2704"
                assert ua["os"]["name"] == "Mac OS X"
                assert ua["os"]["version"] == "10.10.5"
                assert ua["os"]["full"] == "Mac OS X 10.10.5"
                assert ua["device"]["name"] == "Other"


class PipelineDisableOverwriteTest(ElasticTest):
    config_overrides = {
        "register_pipeline_enabled": "true",
        "register_pipeline_overwrite": "false"
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
