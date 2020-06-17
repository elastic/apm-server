import time

from apmserver import integration_test
from apmserver import ClientSideElasticTest, ElasticTest, ExpvarBaseTest, ProcStartupFailureTest
from helper import wait_until
from es_helper import index_smap, index_metric, index_transaction


@integration_test
class Test(ElasticTest):
    def config(self):
        cfg = super(Test, self).config()
        cfg.update({
            "aggregation_enabled": True,
            "aggregation_interval": "1s",
            # Drop unsampled transaction events,
            # to show that we aggregate before they
            # are dropped.
            "sampling_keep_unsampled": False,
        })
        return cfg

    def test_transaction_metrics(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 8)
        self.assert_no_logged_warnings()

        self.wait_for_events('transaction', 3, index=index_transaction)

        metric_docs = self.wait_for_events('metric', 3, index=index_metric)
        for doc in metric_docs:
            # @timestamp is dynamic, so set it to something known.
            doc['_source']['@timestamp'] = '2020-04-14T08:56:03.100Z'
        self.approve_docs('transaction_histogram_metrics', metric_docs)

    def test_rum_transaction_metrics(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans_rum.ndjson"),
                                     self.intake_url, 'transaction', 2)
        self.assert_no_logged_warnings()

        self.wait_for_events('transaction', 1, index=index_transaction)

        metric_docs = self.wait_for_events('metric', 1, index=index_metric)
        for doc in metric_docs:
            # @timestamp is dynamic, so set it to something known.
            doc['_source']['@timestamp'] = '2020-04-14T08:56:03.100Z'
        self.approve_docs('rum_transaction_histogram_metrics', metric_docs)
