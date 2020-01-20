from apmserver import BaseTest, ElasticTest, integration_test
import logging
from elasticsearch import Elasticsearch, NotFoundError, RequestError
from nose.tools import raises
from datetime import datetime, timedelta
import os
import sys
import time
import unittest

sys.path.append(os.path.join(os.path.dirname(__file__), '..',
                             '..', '_beats', 'libbeat', 'tests', 'system'))
from beat.beat import INTEGRATION_TESTS, TestCase, TimeoutError

EVENT_NAMES = ('error', 'span', 'transaction', 'metric', 'profile')

def wait_until(cond, max_timeout=10, poll_interval=0.25, name="cond"):
    """
    Like beat.beat.wait_until but catches exceptions
    In a ElasticTest `cond` will usually be a query, and we need to keep retrying
     eg. on 503 response codes
    """
    assert callable(cond), "First argument of wait_until must be a function"

    start = datetime.now()
    while datetime.now()-start < timedelta(seconds=max_timeout):
        try:
            result = cond()
            if result:
                return result
        except AttributeError as ex:
            raise ex
        except Exception as ex:
            pass
        time.sleep(poll_interval)
    raise TimeoutError("Timeout waiting for '{}' to be true. ".format(name) +
                       "Waited {} seconds.".format(max_timeout))


class IdxMgmt(object):

    def __init__(self, client, index, policies):
        self._client = client
        self._index = index
        self._event_indices = ["{}-{}".format(self._index, e) for e in EVENT_NAMES]
        self.policies = policies

    def indices(self):
        return self._event_indices + [self._index]

    # def clean(self, index=None, policy=None):
    #     self.delete()
    #
    #     if index is None:
    #         index = self.indices()
    #     if policy is None:
    #         policy = self.default_policy
    #
    #     def wait_until_clean():
    #         if self._client.indices.exists(index=index):
    #                 return False
    #         if self._client.indices.exists_template(name=index, ignore=[404]):
    #                 return False
    #
    #         try:
    #             self._client.transport.perform_request('GET', '/_ilm/policy/' + policy)
    #             return False
    #         except NotFoundError:
    #             return True
    #
    #     wait_until(wait_until_clean, name="idxmgmt wait until clean")

    def join(self, base, a):
        return "{}{}".format(base, ",".join(a))

    def delete(self):
        self._client.indices.delete(index=self._index+'*', ignore=[404])
        self._client.indices.delete_template(name=self._index+'*', ignore=[404])
        self.delete_policies()

    def delete_policies(self):
        try:
            self._client.transport.perform_request('DELETE', self.join("/_ilm/policy/", self.policies))
        except NotFoundError:
            pass

    def assert_template(self, loaded=1):
        resp = self._client.indices.get_template(name=self._index, ignore=[404])
        if loaded == 0:
            return self.assert_empty(resp)

        assert len(resp) == loaded, resp
        s, i, l = 'settings', 'index', 'lifecycle'
        assert l not in resp[self._index][s][i]

    def assert_event_template(self, loaded=len(EVENT_NAMES), with_ilm=True):
        resp = self._client.indices.get_template(name=self._index + '*', ignore=[404])

        if self._index in resp:
            loaded += 1
        assert loaded == len(resp), len(resp)

        if loaded <= 1:
            return

        s, i, l = 'settings', 'index', 'lifecycle'
        for idx in self._event_indices:
            if not with_ilm:
                assert i not in resp[idx][s], resp[idx]
                continue
            t = resp[idx][s][i]
            assert l in t, t
            assert t[l]['name'] is not None, t[l]
            assert t[l]['rollover_alias'] == idx, t[l]

    def wait_until_created(self, templates=None, aliases=None, policies=None):
        if templates is None:
            templates = [self._index] + self._event_indices
        wait_until(lambda: len(self._client.indices.get_template(name=templates, ignore=[404])) == len(templates),
                   name="expected templates: {}".format(templates))

        if aliases is None:
            aliases = self._event_indices
        def wait_until_aliases():
            try:
                url = "/_alias/{}".format(",".join(aliases) if aliases else "apm*")
                resp = self._client.transport.perform_request('GET', url)
                return len(resp) == len(aliases)
            except NotFoundError:
                return len(aliases) == 0
        wait_until(lambda: wait_until_aliases(),max_timeout=2,
                   name="expected aliases: {}".format(aliases))

        if policies is None:
            policies = self.policies
        def wait_until_policy():
            try:
                url = "/_ilm/policy/{}".format(",".join(policies) if policies else "apm*")
                resp = self._client.transport.perform_request('GET',  url)
                return len(resp) == len(policies)
            except NotFoundError:
                return len(policies) == 0

        wait_until(lambda: wait_until_policy(),
                   name="expected policies: {}".format(policies))

    def assert_alias(self, loaded=len(EVENT_NAMES)):
        resp = self._client.transport.perform_request('GET', '/_alias/' + self._index + '*')
        if loaded == 0:
            return self.assert_empty(resp)

        assert len(resp) == loaded, resp
        for idx in self._event_indices:
            assert "{}-000001".format(idx) in resp, resp

    def assert_policies(self, loaded=True):
        resp = self._client.transport.perform_request('GET', '/_ilm/policy')
        for p in self.policies:
            assert p in resp if loaded else p not in resp, "policy missing in response: {}".format(p, resp)

    def fetch_policy(self, name):
        return self._client.transport.perform_request('GET', '/_ilm/policy/' + name)

    def assert_docs_written_to_alias(self, alias):
        assert 1 == 2

    def assert_empty(self, a):
        assert len(a) == 0, a


@integration_test
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

    def render_config(self):
        cfg = {"elasticsearch_host": self.get_elasticsearch_url(),
               "file_enabled": "false"}
        self.render_config_template(**cfg)

    def test_setup_default_template_enabled_ilm_auto(self):
        """
        Test setup --index-management when template enabled and ilm auto
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()

    def test_setup_default_template_ilm_setup_disabled(self):
        """
        Test setup --index-management when ilm setup false
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.setup.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template(loaded=0)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_default_policy(loaded=False)

    def test_setup_template_enabled_ilm_disabled(self):
        """
        Test setup --index-management when template enabled and ilm disabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_default_policy(loaded=False)

    def test_setup_template_disabled_ilm_auto(self):
        """
        Test setup --index-management when template disabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=0)
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()

    def test_setup_template_disabled_ilm_disabled(self):
        """
        Test setup --index-management when template disabled and ilm disabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false",
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=0)
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_default_policy(loaded=False)

    def test_enable_ilm(self):
        """
        Test setup --index-management when ilm was enabled and gets disabled
        """
        # load with ilm enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()

        # load with ilm disabled: setup.overwrite is respected
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()

        # load with ilm disabled and setup.overwrite enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false",
                                              "-E", "apm-server.ilm.setup.overwrite=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias()  # aliases do not get deleted
        self.idxmgmt.assert_default_policy()  # policies do not get deleted

    def test_disable(self):
        """
        Test setup --index-management when ilm was disabled and gets enabled
        """
        # load with ilm disabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_default_policy(loaded=False)

        # load with ilm enabled: setup.overwrite is respected
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()

        # load with ilm enabled and setup.overwrite enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true",
                                              "-E", "apm-server.ilm.setup.overwrite=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()

    @raises(RequestError)
    def test_ensure_policy_is_used(self):
        """
        Test setup --index-management actually creates policies that are used
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_default_policy()
        # try deleting policy needs to raise an error as it is in use
        self.idxmgmt.delete_policies()


@integration_test
class TestRunIndexManagementDefault(ElasticTest):
    register_pipeline_disabled = True
    config_overrides = {"queue_flush": 2048}

    def setUp(self):
        super(TestRunIndexManagementDefault, self).setUp()
        self.idxmgmt = IdxMgmt(self.es, self.index_name, [self.policy_name])

    def test_template_loaded(self):
        self.idxmgmt.wait_until_created()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_policies()


@integration_test
class TestRunIndexManagementWithoutILM(ElasticTest):
    register_pipeline_disabled = True
    config_overrides = {"queue_flush": 2048}

    def setUp(self):
        super(TestRunIndexManagementWithoutILM, self).setUp()
        self.idxmgmt = IdxMgmt(self.es, self.index_name, [self.policy_name])

    def start_args(self):
        return {"extra_args": ["-E", "apm-server.ilm.enabled=false"]}

    def test_template_and_ilm_loaded(self):
        self.idxmgmt.wait_until_created(aliases=[], policies=[])
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_policies(loaded=False)


@integration_test
class TestILMConfiguredPolicies(ElasticTest):

    config_overrides = {"queue_flush": 2048, "ilm_policies": True}

    def setUp(self):
        super(TestILMConfiguredPolicies, self).setUp()
        self.custom_policy = "apm-rollover-10-days"
        self.idxmgmt = IdxMgmt(self.es, self.index_name, [self.custom_policy, self.policy_name])

    def test_ilm_loaded(self):
        self.idxmgmt.wait_until_created()
        self.idxmgmt.assert_event_template(with_ilm=True)
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_policies(loaded=True)

        # check out configured policy in apm-server.yml.j2

        # ensure default policy is changed
        policy = self.idxmgmt.fetch_policy(self.policy_name)
        phases = policy[self.policy_name]["policy"]["phases"]
        assert len(phases) == 2
        assert "hot" in phases
        assert "delete" in phases

        # ensure newly configured policy is loaded
        policy = self.idxmgmt.fetch_policy(self.custom_policy)
        phases = policy[self.custom_policy]["policy"]["phases"]
        assert len(phases) == 1
        assert "hot" in phases


@integration_test
class TestRunIndexManagementWithSetupDisabled(ElasticTest):

    config_overrides = {"queue_flush": 2048, "ilm_setup_enabled": "false"}

    def setUp(self):
        super(TestRunIndexManagementWithSetupDisabled, self).setUp()
        self.idxmgmt = IdxMgmt(self.es, self.index_name, [self.policy_name])

    def test_template_and_ilm_loaded(self):
        self.idxmgmt.wait_until_created(templates=[self.index_name], policies=[], aliases=[])
        self.idxmgmt.assert_event_template(loaded=0)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_policies(loaded=False)


@integration_test
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

    def test_setup_default(self):
        """
        Test setup --template default
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd])
        assert exit_code == 0
        self.idxmgmt.assert_template()

        # do not setup ILM when running this command
        self.idxmgmt.assert_event_template(loaded=0)
        self.idxmgmt.assert_alias(loaded=0)
        self.idxmgmt.assert_default_policy(loaded=False)

    def test_setup_template_disabled_ilm_enabled(self):
        """
        Test setup --template when template disabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=0)
