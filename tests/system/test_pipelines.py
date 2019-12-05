import unittest
from apmserver import ElasticTest, SubCommandTest
from beat.beat import INTEGRATION_TESTS, TimeoutError
from elasticsearch import Elasticsearch, NotFoundError
from nose.tools import raises


# APM Server `setup`

@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class SetupPipelinesDefaultTest(SubCommandTest):
    pipeline_name = "apm_user_agent"

    def start_args(self):
        return {
            "logging_args": ["-v", "-d", "*"],
            "extra_args":   ["-e",
                             "setup",
                             "--pipelines"]
        }

    def setUp(self):
        # TODO (gr): consolidate with ElasticTest
        self.es = Elasticsearch([self.get_elasticsearch_url()])
        self.es.ingest.delete_pipeline(id="*")
        super(SetupPipelinesDefaultTest, self).setUp()

    def assert_pipeline_presence(self, should_exist=False):
        try:
            self.es.ingest.get_pipeline(self.pipeline_name)
            present = True
        except NotFoundError:
            present = False

        assert should_exist == present, "expected {}pipelines".format("" if should_exist else "no ")

    def test_setup_pipelines(self):
        self.assert_pipeline_presence(True)
        assert self.log_contains("Pipeline successfully registered: apm_user_agent")
        assert self.log_contains("Registered Ingest Pipelines successfully.")


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class SetupPipelinesDisabledTest(SetupPipelinesDefaultTest):
    def config(self):
        cfg = super(SetupPipelinesDisabledTest, self).config()
        cfg.update({
            "register_pipeline_enabled": "false",
        })
        return cfg

    def test_setup_pipelines(self):
        self.assert_pipeline_presence(False)
        assert self.log_contains("No pipeline callback registered")


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class PipelineRegisterTest(ElasticTest):
    def test_default_pipelines_registered(self):
        pipelines = [
            ("apm_user_agent", "Add user agent information for APM events"),
            ("apm_user_geo", "Add user geo information for APM events"),
            ("apm", "Default enrichment for APM events"),
        ]
        loaded_msg = "Pipeline successfully registered"
        self.wait_until(lambda: self.log_contains(loaded_msg), name=loaded_msg)

        for pipeline_id, pipeline_desc in pipelines:
            pipeline = self.wait_until(lambda: self.es.ingest.get_pipeline(id=pipeline_id),
                                       name="fetching pipeline {}".format(pipeline_id))
            assert pipeline[pipeline_id]['description'] == pipeline_desc

    def test_pipeline_applied(self):
        # setup
        self.wait_until_pipelines_registered()
        self.wait_until_ilm_setup()
        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 3)

        entries = self.es.search(index=self.index_transaction)['hits']['hits']
        ua_found = False
        for e in entries:
            src = e['_source']
            if 'user_agent' in src:
                ua_found = True
                ua = src['user_agent']
                assert ua is not None
                assert ua["original"] == "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 " \
                                         "(KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36, Mozilla Chrome Edge"
                assert ua["name"] == "Chrome"
                assert ua["version"] == "51.0.2704.103"
                assert ua["os"]["name"] == "Mac OS X"
                assert ua["os"]["version"] == "10.10.5"
                assert ua["os"]["full"] == "Mac OS X 10.10.5"
                assert ua["device"]["name"] == "Other"
        assert ua_found


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class PipelineDisabledTest(ElasticTest):
    config_overrides = {"disable_pipeline": True}

    def test_pipeline_not_applied(self):
        self.wait_until_ilm_setup()
        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 3)
        uaFound = False
        entries = self.es.search(index=self.index_transaction)['hits']['hits']
        for e in entries:
            src = e['_source']
            if 'user_agent' in src:
                uaFound = True
                ua = src['user_agent']
                assert ua is not None
                assert ua["original"] == "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 " \
                                         "(KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36, Mozilla Chrome Edge"
                assert 'name' not in ua
        assert uaFound


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class PipelinesConfigurationNoneTest(ElasticTest):
    config_overrides = {"disable_pipelines": True}

    def test_pipeline_not_applied(self):
        self.wait_until_ilm_setup()
        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 3)

        entries = self.es.search(index=self.index_transaction)['hits']['hits']
        uaFound = False
        for e in entries:
            src = e['_source']
            if 'user_agent' in src:
                uaFound = True
                ua = src['user_agent']
                assert ua is not None
                assert ua["original"] == "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 " \
                                         "(KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36, Mozilla Chrome Edge"
                assert 'name' not in ua
        assert uaFound


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class MissingPipelineTest(ElasticTest):
    config_overrides = {"register_pipeline_enabled": "false"}

    @raises(TimeoutError)
    def test_pipeline_not_registered(self):
        self.wait_until(lambda: self.log_contains("No pipeline callback registered"),
                        name="pipeline callback not registered")
        self.wait_until_ilm_setup()
        # ensure events get stored properly nevertheless
        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 3)


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class PipelineDefaultDisableRegisterOverwriteTest(ElasticTest):

    config_overrides = {
        "register_pipeline_overwrite": "false",
        "queue_flush": 2048,
    }

    def setUp(self):
        super(PipelineDefaultDisableRegisterOverwriteTest, self).setUp()
        es = Elasticsearch([self.get_elasticsearch_url()])
        # Write empty default pipeline
        es.ingest.put_pipeline(
            id="apm",
            body={"description": "empty apm test pipeline", "processors": []})
        self.wait_until(lambda: es.ingest.get_pipeline("apm"), name="apm ingest pipeline created")

    def test_pipeline_not_overwritten(self):
        loaded_msg = "Pipeline already registered"
        self.wait_until(lambda: self.log_contains(loaded_msg), name=loaded_msg)


@unittest.skipUnless(INTEGRATION_TESTS, "integration test")
class PipelineEnableRegisterOverwriteTest(ElasticTest):
    config_overrides = {
        "register_pipeline_overwrite": "true",
        "queue_flush": 2048,
    }

    def setUp(self):
        super(PipelineEnableRegisterOverwriteTest, self).setUp()
        es = Elasticsearch([self.get_elasticsearch_url()])
        # Write empty default pipeline
        es.ingest.put_pipeline(
            id="apm",
            body={"description": "empty apm test pipeline", "processors": []})
        self.wait_until(lambda: es.ingest.get_pipeline("apm"), name="apm ingest pipeline created")

    def test_pipeline_overwritten(self):
        loaded_msg = "Registered Ingest Pipelines successfully"
        self.wait_until(lambda: self.log_contains(loaded_msg))
        pipeline_id = "apm"
        desc = "Default enrichment for APM events"
        self.wait_until(lambda: self.es.ingest.get_pipeline(id=pipeline_id)[pipeline_id]['description'] == desc,
                        name="fetching pipeline {}".format(pipeline_id))
