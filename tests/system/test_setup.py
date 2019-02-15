import shutil
import os
import unittest

from elasticsearch import Elasticsearch
from apmserver import BaseTest
from beat.beat import INTEGRATION_TESTS


class SetupTemplateDefaultTest(BaseTest):
    """
    Test setup template subcommand with default option.
    """

    def get_elasticsearch_url(self):
        """
        Returns an elasticsearch.Elasticsearch instance built from the
        env variables like the integration tests.
        """
        return "http://{host}:{port}".format(
            host=os.getenv("ES_HOST", "localhost"),
            port=os.getenv("ES_PORT", "9200"),
    )

    def setUp(self):
        super(BaseTest, self).setUp()

        self.elasticsearch_url = self.get_elasticsearch_url()
        self.es = Elasticsearch([self.elasticsearch_url])

        try:
            self.es.indices.delete_template('apm-*')
        except:
            pass
        assert self.es.indices.exists_template(name='apm-*') == False

        shutil.copy(self.beat_path + "/_meta/beat.yml",
                os.path.join(self.working_dir, "apm-server.yml"))

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_setup_default_template(self):
        """
        Test setup default template
        """
        exit_code = self.run_beat(
            logging_args=["-v", "-d", "*"],
            extra_args=["-e",
                        "setup",
                        "-template",
                        "-path.config", self.working_dir],
            config="apm-server.yml")
        assert exit_code == 0

        print(self.es.cat.templates(name='apm-*', h='name'))
        assert self.es.indices.exists_template(name='apm-*') == True
        assert self.log_contains("Loaded index template")
        assert self.log_contains("Index setup complete")
        self.assertNotRegexpMatches(self.get_log(), "ILM")


    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_setup_template(self):
        """
        Test setup default template
        """
        exit_code = self.run_beat(
            logging_args=["-v", "-d", "*"],
            extra_args=["-e",
                        "setup",
                        "-template",
                        "-path.config", self.working_dir,
                        "-E", "setup.template.overwrite=true"],
            config="apm-server.yml")

        assert exit_code == 0
        assert self.es.indices.exists_template(name='apm-*') == True
        assert self.log_contains("Loaded index template")
        assert self.log_contains("Index setup complete")
        self.assertNotRegexpMatches(self.get_log(), "ILM")

