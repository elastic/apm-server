import re

import requests

from apmserver import integration_test
from apmserver import ElasticTest


@integration_test
class Test(ElasticTest):
    jaeger_http_host = "localhost:14268"

    def setUp(self):
        super(Test, self).setUp()
        self.wait_until(lambda: self.log_contains("Listening for Jaeger HTTP"), name="Jaeger HTTP listener started")

        # Extract the Jaeger HTTP server address.
        match = re.search("Listening for Jaeger HTTP requests on: (.*)$", self.get_log(), re.MULTILINE)
        listen_addr = match.group(1)
        self.jaeger_http_url = "http://{}/{}".format(listen_addr, 'api/traces')

    def config(self):
        cfg = super(Test, self).config()
        cfg.update({
            "jaeger_http_enabled": "true",
            "jaeger_http_host": "localhost:0", # Listen on a dynamic port
        })
        return cfg

    def test_jaeger_http(self):
        """
        This test sends a Jaeger span in Thrift encoding over HTTP, and verifies that it is indexed.
        """
        jaeger_span_thrift = self.get_testdata_path('jaeger', 'span.thrift')
        self.load_docs_with_template(jaeger_span_thrift, self.jaeger_http_url, 'transaction', 1,
                                     extra_headers={"content-type": "application/vnd.apache.thrift.binary"})
        self.assert_no_logged_warnings()

        rs = self.es.search(index=self.index_transaction)
        assert rs['hits']['total']['value'] == 1, "found {} documents".format(rs['count'])
        self.approve_docs('jaeger_span', rs['hits']['hits'], 'transaction')
