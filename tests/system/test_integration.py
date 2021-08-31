import time

from apmserver import integration_test
from apmserver import ClientSideElasticTest, ElasticTest, ExpvarBaseTest, ProcStartupFailureTest
from helper import wait_until
from es_helper import index_smap, index_metric, index_transaction, index_error, index_span, index_onboarding, index_name


@integration_test
class Test(ElasticTest):

    def test_template(self):
        """
        This test starts the beat and checks that the template has been loaded to ES
        """
        wait_until(lambda: self.es.indices.exists(index_onboarding))
        templates = self.es.indices.get_template(index_name)
        assert len(templates) == 1
        t = templates[index_name]
        total_fields_limit = t['settings']['index']['mapping']['total_fields']['limit']
        assert total_fields_limit == "2000", total_fields_limit

    def test_tags_type(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()
        mappings = self.es.indices.get_field_mapping(index=index_transaction, fields="context.tags.*")
        for name, metric in mappings["{}-000001".format(index_transaction)]["mappings"].items():
            fullname = metric["full_name"]
            for mapping in metric["mapping"].values():
                mtype = mapping["type"]
                if fullname.startswith("context.tags.bool"):
                    assert mtype == "boolean", name + " mapped as " + mtype + ", not boolean"
                elif fullname.startswith("context.tags.number"):
                    assert mtype == "scaled_float", name + " mapped as " + mtype + ", not scaled_float"
                else:
                    assert mtype == "keyword", name + " mapped as " + mtype + ", not keyword"

    def test_mark_type(self):
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()
        mappings = self.es.indices.get_field_mapping(index=index_transaction, fields="transaction.marks.*")
        for name, metric in mappings["{}-000001".format(index_transaction)]["mappings"].items():
            for mapping in metric["mapping"].values():
                mtype = mapping["type"]
                assert mtype == "scaled_float", name + " mapped as " + mtype + ", not scaled_float"

    def test_load_docs_with_template_and_add_transaction(self):
        """
        This test starts the beat with a loaded template and sends transaction data to elasticsearch.
        It verifies that all data make it into ES, means data is compatible with the template
        and data are in expected format.
        """
        self.load_docs_with_template(self.get_payload_path("transactions_spans.ndjson"),
                                     self.intake_url, 'transaction', 9)
        self.assert_no_logged_warnings()

        # compare existing ES documents for transactions with new ones
        transaction_docs = self.wait_for_events('transaction', 4, index=index_transaction)
        self.approve_docs('transaction', transaction_docs)

        # compare existing ES documents for spans with new ones
        span_docs = self.wait_for_events('transaction', 5, index=index_span)
        self.approve_docs('spans', span_docs)

    def test_load_docs_with_template_and_add_error(self):
        """
        This test starts the beat with a loaded template and sends error data to elasticsearch.
        It verifies that all data make it into ES means data is compatible with the template.
        """
        self.load_docs_with_template(self.get_error_payload_path(), self.intake_url, 'error', 4)
        self.assert_no_logged_warnings()

        # compare existing ES documents for errors with new ones
        error_docs = self.wait_for_events('error', 4, index=index_error)
        self.approve_docs('error', error_docs)

        self.check_backend_error_sourcemap(index_error, count=4)

    def test_load_docs_with_template_and_add_metricset(self):
        # NOTE(axw) this test is redundant with systemtest.TestApprovedMetrics,
        # but we use the output of this test for generating documentation.
        self.load_docs_with_template(self.get_metricset_payload_path(), self.intake_url, 'metric', 5)
        self.assert_no_logged_warnings()

        # compare existing ES documents for metricsets with new ones
        metricset_docs = self.wait_for_events('metric', 5, index=index_metric)
        self.approve_docs('metricset', metricset_docs)


@integration_test
class EnrichEventIntegrationTest(ClientSideElasticTest):
    def test_backend_error(self):
        # for backend events library_frame information should not be changed,
        # as no regex pattern is defined.
        self.load_docs_with_template(self.get_backend_error_payload_path(),
                                     self.backend_intake_url,
                                     'error',
                                     4)
        self.check_library_frames({"true": 1, "false": 0, "empty": 3}, index_error)

    def test_rum_error(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.check_library_frames({"true": 5, "false": 0, "empty": 1}, index_error)

    def test_backend_transaction(self):
        # for backend events library_frame information should not be changed,
        # as no regex pattern is defined.
        self.load_docs_with_template(self.get_backend_transaction_payload_path(),
                                     self.backend_intake_url,
                                     'transaction',
                                     9)
        self.check_library_frames({"true": 1, "false": 0, "empty": 1}, index_span)

    def test_rum_transaction(self):
        self.load_docs_with_template(self.get_transaction_payload_path(),
                                     self.intake_url,
                                     'transaction',
                                     2)
        self.check_library_frames({"true": 1, "false": 0, "empty": 1}, index_span)

    def test_enrich_backend_event(self):
        self.load_docs_with_template(self.get_backend_transaction_payload_path(),
                                     self.backend_intake_url, 'transaction', 9)

        rs = self.es.search(index=index_transaction)

        assert "ip" in rs['hits']['hits'][0]["_source"]["host"], rs['hits']

    def test_enrich_rum_event(self):
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)

        rs = self.es.search(index=index_error)

        hits = rs['hits']['hits']
        for hit in hits:
            assert "user_agent" in hit["_source"], rs['hits']
            assert "original" in hit["_source"]["user_agent"], rs['hits']
            assert "ip" in hit["_source"]["client"], rs['hits']

    def test_grouping_key_for_error(self):
        # upload the same error, once via rum, once via backend endpoint
        # check they don't have the same grouping key, as the
        # `rum.exclude_from_grouping` should only be applied to the rum error.
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     1)
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.backend_intake_url,
                                     'error',
                                     2)

        rs = self.es.search(index=index_error)
        docs = rs['hits']['hits']
        grouping_key1 = docs[0]["_source"]["error"]["grouping_key"]
        grouping_key2 = docs[1]["_source"]["error"]["grouping_key"]
        assert grouping_key1 != grouping_key2

    def check_library_frames(self, library_frames, index_name):
        rs = self.es.search(index=index_name)
        l_frames = {"true": 0, "false": 0, "empty": 0}
        for doc in rs['hits']['hits']:
            if "error" in doc["_source"]:
                err = doc["_source"]["error"]
                for exception in err.get("exception", []):
                    self.count_library_frames(exception, l_frames)
                if "log" in err:
                    self.count_library_frames(err["log"], l_frames)
            elif "span" in doc["_source"]:
                span = doc["_source"]["span"]
                self.count_library_frames(span, l_frames)
        assert l_frames == library_frames, "found {}, expected {}".format(
            l_frames, library_frames)

    @staticmethod
    def count_library_frames(doc, lf):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            if "library_frame" in frame:
                k = "true" if frame["library_frame"] else "false"
                lf[k] += 1
            else:
                lf["empty"] += 1


@integration_test
class ILMDisabledIntegrationTest(ElasticTest):
    config_overrides = {"ilm_enabled": "false"}

    def test_override_indices_config(self):
        # load error and transaction document to ES
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     4,
                                     query_index="{}-2017.05.09".format(index_error))


class OverrideIndicesTest(ElasticTest):
    def config(self):
        cfg = super(OverrideIndicesTest, self).config()
        cfg.update({"override_index": index_name,
                    "override_template": index_name})
        return cfg


@integration_test
class OverrideIndicesIntegrationTest(OverrideIndicesTest):
    # default ILM=auto disables ILM when custom indices given

    def test_override_indices_config(self):
        # load error and transaction document to ES
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     4,
                                     query_index=index_name)
        self.load_docs_with_template(self.get_payload_path("transactions_spans_rum.ndjson"),
                                     self.intake_url,
                                     'transaction',
                                     2,
                                     query_index=index_name)

        # check that every document is indexed once in the expected index (incl.1 onboarding doc)
        assert 4+2+1 == self.es.count(index=index_name)['count']


@integration_test
class OverrideIndicesILMFalseIntegrationTest(OverrideIndicesTest):
    config_overrides = {"ilm_enabled": "false"}

    def test_override_indices_config(self):
        # load error and transaction document to ES
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     4,
                                     query_index=index_name)
        assert 4+1 == self.es.count(index=index_name)['count']


@integration_test
class OverrideIndicesILMTrueIntegrationTest(OverrideIndicesTest):
    config_overrides = {"ilm_enabled": "true"}

    def test_override_indices_config(self):
        # load error and transaction document to ES
        self.load_docs_with_template(self.get_error_payload_path(),
                                     self.intake_url,
                                     'error',
                                     4,
                                     query_index=self.ilm_index(index_error))
        assert 4 == self.es.count(index=self.ilm_index(index_error))['count']


@integration_test
class OverrideIndicesFailureIntegrationTest(ProcStartupFailureTest):
    config_overrides = {
        "override_index": "apm-foo",
        "elasticsearch_host": "localhost:8200",
        "file_enabled": "false",
    }

    def test_template_setup_error(self):
        loaded_msg = "Exiting: `setup.template.name` and `setup.template.pattern` have to be set"
        wait_until(lambda: self.log_contains(loaded_msg), max_timeout=5)


@integration_test
class ExpvarDisabledIntegrationTest(ExpvarBaseTest):
    config_overrides = {"expvar_enabled": "false"}

    def test_expvar_exists(self):
        """expvar disabled, should 404"""
        r = self.get_debug_vars()
        assert r.status_code == 404, r.status_code


@integration_test
class ExpvarEnabledIntegrationTest(ExpvarBaseTest):
    config_overrides = {"expvar_enabled": "true"}

    def test_expvar_exists(self):
        """expvar enabled, should 200"""
        r = self.get_debug_vars()
        assert r.status_code == 200, r.status_code


@integration_test
class ExpvarCustomUrlIntegrationTest(ExpvarBaseTest):
    config_overrides = {"expvar_enabled": "true", "expvar_url": "/foo"}
    expvar_url = ExpvarBaseTest.expvar_url.replace("/debug/vars", "/foo")

    def test_expvar_exists(self):
        """expvar enabled, should 200"""
        r = self.get_debug_vars()
        assert r.status_code == 200, r.status_code
