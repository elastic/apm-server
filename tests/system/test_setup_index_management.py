import os
from nose.plugins.attrib import attr
import unittest
import logging
from elasticsearch import Elasticsearch, NotFoundError, RequestError
from apmserver import BaseTest, ElasticTest
from nose.tools import raises

INTEGRATION_TESTS = os.environ.get('INTEGRATION_TESTS', False)


class IdxMgmt(object):

    def __init__(self, client, index):
        self._client = client
        self._index = index
        self._event_indices = ["{}-{}".format(self._index, e) for e in ['error', 'span', 'transaction', 'metric']]

    def indices(self):
        return self._event_indices + [self._index]

    def delete(self):
        self._client.indices.delete(index=self._index+'*', ignore=[404])
        self._client.indices.delete_template(name=self._index+'*', ignore=[404])
        self.delete_policies()

    def delete_policies(self):
        for e in self._event_indices:
            try:
                self._client.transport.perform_request('DELETE', '/_ilm/policy/' + e)
            except NotFoundError:
                pass

    def assert_template(self, loaded=False, with_ilm=False):
        resp = self._client.indices.get_template(name=self._index + '*', ignore=[404])
        if not loaded:
            return self.assert_empty(resp)

        assert len(resp) == len(self.indices()), resp
        s, i, l = 'settings', 'index', 'lifecycle'
        assert l not in resp[self._index][s][i]
        for idx in self._event_indices:
            if not with_ilm:
                assert i not in resp[idx][s], resp[idx]
                continue
            t = resp[idx][s][i]
            assert l in t, t
            assert t[l]['name'] == idx, t[l]
            assert t[l]['rollover_alias'] == idx, t[l]

    def assert_alias(self, loaded=False):
        resp = self._client.transport.perform_request('GET', '/_alias/' + self._index + '*')
        if not loaded:
            return self.assert_empty(resp)

        assert len(resp) == len(self._event_indices), resp
        for idx in self._event_indices:
            assert "{}-000001".format(idx) in resp, resp

    def assert_policies(self, loaded=False):
        resp = self._client.transport.perform_request('GET', '/_ilm/policy')
        for idx in self._event_indices:
            assert idx in resp if loaded else idx not in resp

    def assert_docs_written_to_alias(self, alias):
        assert 1 == 2

    def assert_empty(self, a):
        assert len(a) == 0, a


class TestCommandSetupIndexManagement(BaseTest):
    """
    Test beat command `setup --index-management`
    """

    def setUp(self):
        super(TestCommandSetupIndexManagement, self).setUp()
        self.cmd = "--index-management"

        self._es = Elasticsearch([self.get_elasticsearch_url()])
        logging.getLogger("elasticsearch").setLevel(logging.ERROR)
        logging.getLogger("urllib3").setLevel(logging.WARNING)

        self.idxmgmt = IdxMgmt(self._es, self.index_name)
        self.idxmgmt.delete()
        self.render_config()

    def tearDown(self):
        self.idxmgmt.delete()

    def render_config(self):
        cfg = {"elasticsearch_host": self.get_elasticsearch_url(),
               "file_enabled": "false"}
        self.render_config_template(**cfg)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_default_template_enabled_ilm_auto(self):
        """
        Test setup --index-management when template enabled and ilm auto
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=True)
        self.idxmgmt.assert_policies(loaded=True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_enabled_ilm_disabled(self):
        """
        Test setup --index-management when template enabled and ilm disabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_enabled_ilm_enabled(self):
        """
        Test setup --index-management when template enabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=True)
        self.idxmgmt.assert_policies(loaded=True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_disabled_ilm_auto(self):
        """
        Test setup --index-management when template disabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=False, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=True)
        self.idxmgmt.assert_policies(loaded=True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_disabled_ilm_disabled(self):
        """
        Test setup --index-management when template disabled and ilm disabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false",
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=False)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_enable_ilm(self):
        """
        Test setup --index-management when ilm was enabled and gets disabled
        """
        # load with ilm enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=True)
        self.idxmgmt.assert_policies(loaded=True)

        # load with ilm disabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=False)
        self.idxmgmt.assert_policies(loaded=True)  # policies do not get deleted
        self.idxmgmt.assert_alias(loaded=True)  # aliases do not get deleted

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_disable(self):
        """
        Test setup --index-management when ilm was disabled and gets enabled
        """
        # load with ilm disabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=False)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

        # load with ilm enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_policies(loaded=True)
        self.idxmgmt.assert_alias(loaded=True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    @raises(RequestError)
    def test_ensure_policy_is_used(self):
        """
        Test setup --index-management actually creates policies that are used
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=True)
        self.idxmgmt.assert_policies(loaded=True)
        # try deleting policy needs to raise an error as it is in use
        self.idxmgmt.delete_policies()


class TestRunIndexManagementDefault(ElasticTest):
    def setUp(self):
        super(TestRunIndexManagementDefault, self).setUp()

        self.idxmgmt = IdxMgmt(self.es, self.index_name)
        self.idxmgmt.delete()

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_template_loaded(self):
        self.wait_until(lambda: self.log_contains("Finished index management setup."),
                        max_timeout=5)
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=True)
        self.idxmgmt.assert_policies(loaded=True)


class TestRunIndexManagementWithoutILM(ElasticTest):
    def setUp(self):
        super(TestRunIndexManagementWithoutILM, self).setUp()

        self.idxmgmt = IdxMgmt(self.es, self.index_name)
        self.idxmgmt.delete()

    def start_args(self):
        return {"extra_args": ["-E", "apm-server.ilm.enabled=false"]}

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    def test_template_and_ilm_loaded(self):
        self.wait_until(lambda: self.log_contains("Finished index management setup."),
                        max_timeout=5)
        self.idxmgmt.assert_template(loaded=True, with_ilm=False)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)


class TestCommandSetupTemplate(BaseTest):
    """
    Test beat command `setup --template`
    """

    def setUp(self):
        super(TestCommandSetupTemplate, self).setUp()
        self.cmd = "--template"

        self._es = Elasticsearch([self.get_elasticsearch_url()])
        logging.getLogger("elasticsearch").setLevel(logging.ERROR)
        logging.getLogger("urllib3").setLevel(logging.WARNING)

        self.idxmgmt = IdxMgmt(self._es, self.index_name)
        self.idxmgmt.delete()
        self.render_config()

    def render_config(self):
        cfg = {"elasticsearch_host": self.get_elasticsearch_url(),
               "file_enabled": "false"}
        self.render_config_template(**cfg)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_default(self):
        """
        Test setup --template default
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_enabled_ilm_disabled(self):
        """
        Test setup --template when template enabled and ilm disabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_enabled_ilm_enabled(self):
        """
        Test setup --template when template enabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_disabled_ilm_enabled(self):
        """
        Test setup --template when template disabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=False)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_disabled_ilm_disabled(self):
        """
        Test setup --template when template disabled and ilm disabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false",
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=False)
        self.idxmgmt.assert_alias(loaded=False)
        self.idxmgmt.assert_policies(loaded=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_enable_ilm(self):
        """
        Test setup --template when ilm was enabled and gets disabled
        """
        self.render_config()

        # load with ilm enabled
        exit_code = self.run_beat(logging_args=["-v", "-d", "*"],
                                  extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)

        # load with ilm disabled
        exit_code = self.run_beat(logging_args=["-v", "-d", "*"],
                                  extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=False)

    @unittest.skipUnless(INTEGRATION_TESTS, "integration test")
    @attr('integration')
    def test_setup_template_with_opts(self):
        """
        Test setup --index-management when ilm was disabled and gets enabled
        """
        self.render_config()

        # load with ilm disabled
        exit_code = self.run_beat(logging_args=["-v", "-d", "*"],
                                  extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=False)

        # load with ilm enabled
        exit_code = self.run_beat(logging_args=["-v", "-d", "*"],
                                  extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=True, with_ilm=True)
