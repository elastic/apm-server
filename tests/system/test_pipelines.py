import datetime
import pytest
from apmserver import ElasticTest, SubCommandTest, TimeoutError, integration_test
from elasticsearch import Elasticsearch
from es_helper import wait_until_pipelines_deleted, wait_until_pipelines
from helper import wait_until
from es_helper import index_transaction

# APM Server `setup`

# TRANSACTION_COUNT defines the number of transactions in testdata/transactions.ndjson.
TRANSACTION_COUNT = 5


@integration_test
class SetupCmdPipelinesDefaultTest(SubCommandTest):
    """
    Registers pipelines by default when running `setup --pipelines` command
    """

    def start_args(self):
        return {
            "logging_args": ["-v", "-d", "*"],
            "extra_args":   ["-e",
                             "setup",
                             "--pipelines"]
        }

    def setUp(self):
        # ensure environment is clean before cmd is run
        self.es = Elasticsearch([self.get_elasticsearch_url()])
        wait_until_pipelines_deleted(self.es)
        # pipelines are setup when running the command
        super(SetupCmdPipelinesDefaultTest, self).setUp()

    def test_setup_pipelines(self):
        assert self.log_contains("Pipeline successfully registered: apm_user_agent")
        assert self.log_contains("Registered Ingest Pipelines successfully.")
        wait_until_pipelines(self.es)


@integration_test
class SetupCmdPipelinesDisabledTest(SetupCmdPipelinesDefaultTest):
    """
    Does not register pipelines when disabled via configuration and running `setup --pipelines` command
    """

    def config(self):
        cfg = super(SetupCmdPipelinesDisabledTest, self).config()
        cfg.update({"register_pipeline_enabled": "false"})
        return cfg

    def test_setup_pipelines(self):
        assert self.log_contains("Pipeline registration disabled")
        wait_until_pipelines(self.es, [])


@integration_test
class PipelineRegisterTest(ElasticTest):
    """
    Registers pipelines by default when starting apm-server
    """

    def test_pipeline_registered_and_applied(self):
        wait_until_pipelines(self.es)
        # setup
        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 4)

        entries = self.es.search(index=index_transaction)['hits']['hits']
        ua_found = False
        for e in entries:
            src = e['_source']

            # ingest timestamp pipeline
            ingest_ts = src.get("event", {}).get("ingested")
            assert datetime.datetime.strptime(ingest_ts[:19], "%Y-%m-%dT%H:%M:%S")

            # user agent pipeline
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
                assert ua["device"]["name"] == "Mac"
        assert ua_found


@integration_test
class PipelineConfigurationNoneTest(ElasticTest):
    """
    Registers pipelines, but does not apply them on data ingestion
    """
    config_overrides = {"disable_pipeline": True}

    def test_pipeline_not_applied(self):
        wait_until_pipelines(self.es)
        self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                     self.intake_url, 'transaction', 4)
        uaFound = False
        entries = self.es.search(index=index_transaction)['hits']['hits']
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


@integration_test
class PipelineDisableRegisterTest(ElasticTest):
    """
    Cannot ingest data when pipeline is not registered
    """
    config_overrides = {"register_pipeline_enabled": "false"}

    def test_pipeline_not_registered(self):
        wait_until_pipelines(self.es, [])
        # events do not get stored when pipeline is missing
        with pytest.raises(TimeoutError):
            self.load_docs_with_template(self.get_payload_path("transactions.ndjson"),
                                         self.intake_url, 'transaction', TRANSACTION_COUNT)


class PipelineOverwriteBase(ElasticTest):
    def setUp(self):
        # ensure pipelines do not get deleted on APM Server startup, otherwise `overwrite` flag cannot be tested
        self.skip_clean_pipelines = True

        # Ensure all pipelines are deleted before test
        es = Elasticsearch([self.get_elasticsearch_url()])
        wait_until_pipelines_deleted(es)

        # Ensure `apm` pipeline is already registered in ES before APM Server is started
        self.pipeline_apm = "apm"
        es.ingest.put_pipeline(id=self.pipeline_apm, body={"description": "empty apm test pipeline", "processors": []})
        wait_until_pipelines(es, ["apm"])

        # When starting APM Server pipeline `apm` is already registered, the other pipelines are not
        super(PipelineOverwriteBase, self).setUp()


@integration_test
class PipelineDisableRegisterOverwriteTest(PipelineOverwriteBase):
    """
    Does not overwrite existing pipelines when overwrite is disabled (default)
    """
    config_overrides = {"queue_flush": 2048}

    def test_pipeline_not_overwritten(self):
        loaded_msg = "Pipeline already registered: apm"
        wait_until(lambda: self.log_contains(loaded_msg), name=loaded_msg)
        desc = "empty apm test pipeline"
        wait_until(lambda: self.es.ingest.get_pipeline(id=self.pipeline_apm)[self.pipeline_apm]['description'] == desc,
                   name="fetching pipeline {}".format(self.pipeline_apm))


@integration_test
class PipelineEnableRegisterOverwriteTest(PipelineOverwriteBase):
    """
    Overwrites existing pipelines when enabled
    """
    config_overrides = {
        "register_pipeline_overwrite": "true",
        "queue_flush": 2048,
    }

    def test_pipeline_overwritten(self):
        desc = "Default enrichment for APM events"
        wait_until(lambda: self.es.ingest.get_pipeline(id=self.pipeline_apm)[self.pipeline_apm]['description'] == desc,
                   name="fetching pipeline {}".format(self.pipeline_apm))
