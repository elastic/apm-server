import time

from apmserver import integration_test
from apmserver import ClientSideElasticTest, ElasticTest, ExpvarBaseTest, ProcStartupFailureTest
from helper import wait_until
from es_helper import index_smap, index_metric, index_transaction


@integration_test
class TestKeepUnsampled(ElasticTest):
    def config(self):
        cfg = super(TestKeepUnsampled, self).config()
        cfg.update({"sampling_keep_unsampled": True})
        return cfg

    def test(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()
        docs = self.wait_for_events('transaction', 4, index=index_transaction)
        self.approve_docs('keep_unsampled_transactions', docs)


@integration_test
class TestDropUnsampled(ElasticTest):
    def config(self):
        cfg = super(TestDropUnsampled, self).config()
        cfg.update({
            "sampling_keep_unsampled": False,
            # Enable aggregation to avoid a warning.
            "aggregation_enabled": True,
        })
        return cfg

    def test(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 8)
        self.assert_no_logged_warnings()
        docs = self.wait_for_events('transaction', 3, index=index_transaction)
        self.approve_docs('drop_unsampled_transactions', docs)


@integration_test
class TestConfigWarning(ElasticTest):
    def config(self):
        cfg = super(TestConfigWarning, self).config()
        cfg.update({
            "sampling_keep_unsampled": False,
            # Disable aggregation to force a warning.
            "aggregation_enabled": False,
        })
        return cfg

    def test(self):
        expected = "apm-server.sampling.keep_unsampled and apm-server.aggregation.transactions.enabled are both false, which will lead to incorrect metrics being reported in the APM UI"
        self.assertIn(expected, self.get_log())
