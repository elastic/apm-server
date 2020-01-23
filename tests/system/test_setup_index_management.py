from apmserver import BaseTest, ElasticTest, integration_test
import logging
from elasticsearch import Elasticsearch, NotFoundError, RequestError
from nose.tools import raises
from es_helper import cleanup, wait_until_policies, wait_until_aliases, wait_until_templates
from es_helper import default_policy, index_name

EVENT_NAMES = ('error', 'span', 'transaction', 'metric', 'profile')


class IdxMgmt(object):

    def __init__(self, client, index, policies=[default_policy]):
        self._client = client
        self._index = index
        self._event_indices = ["{}-{}".format(self._index, e) for e in EVENT_NAMES]
        self.policies = policies

    def wait_until_created(self, templates=None, aliases=None, policies=None):
        if templates is None:
            templates = [self._index] + self._event_indices
        if aliases is None:
            aliases = self._event_indices
        if policies is None:
            policies = [default_policy]
        wait_until_templates(self._client, templates)
        wait_until_aliases(self._client, aliases)
        wait_until_policies(self._client, policies)

    def assert_template(self, loaded=1):
        resp = self._client.indices.get_template(name=self._index, ignore=[404])
        assert len(resp) == loaded, resp
        if loaded > 0:
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

    def assert_alias(self, loaded=len(EVENT_NAMES)):
        resp = self._client.transport.perform_request('GET', '/_alias/' + self._index + '*')
        assert len(resp) == loaded, resp
        if loaded > 0:
            for idx in self._event_indices:
                assert "{}-000001".format(idx) in resp, resp

    def assert_policies(self, loaded=True):
        resp = self._client.transport.perform_request('GET', '/_ilm/policy')
        for p in self.policies:
            if loaded:
                assert p in resp, "policy {} missing in response: {}".format(p, resp)
            else:
                assert p not in resp, "expected policy {} not to be in response: {}".format(p, resp)

    def fetch_policy(self, name):
        return self._client.transport.perform_request('GET', '/_ilm/policy/' + name)


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

        self.idxmgmt = IdxMgmt(self._es, index_name)
        cleanup(self._es, delete_pipelines=[])
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
        self.idxmgmt.assert_policies()

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
        self.idxmgmt.assert_policies(loaded=False)

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
        self.idxmgmt.assert_policies(loaded=False)

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
        self.idxmgmt.assert_policies()

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
        self.idxmgmt.assert_policies(loaded=False)

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
        self.idxmgmt.assert_policies()

        # load with ilm disabled: setup.overwrite is respected
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_policies()

        # load with ilm disabled and setup.overwrite enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=false",
                                              "-E", "apm-server.ilm.setup.overwrite=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias()  # aliases do not get deleted
        self.idxmgmt.assert_policies()  # policies do not get deleted

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
        self.idxmgmt.assert_policies(loaded=False)

        # load with ilm enabled: setup.overwrite is respected
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true"])
        assert exit_code == 0
        self.idxmgmt.assert_event_template(with_ilm=False)
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_policies()

        # load with ilm enabled and setup.overwrite enabled
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "apm-server.ilm.enabled=true",
                                              "-E", "apm-server.ilm.setup.overwrite=true"])
        assert exit_code == 0
        self.idxmgmt.assert_template()
        self.idxmgmt.assert_event_template()
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_policies()

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
        self.idxmgmt.assert_policies()
        # try deleting policy needs to raise an error as it is in use
        self._es.transport.perform_request('DELETE', "/_ilm/policy/{}".format(default_policy))


@integration_test
class TestRunIndexManagementDefault(ElasticTest):
    register_pipeline_disabled = True
    config_overrides = {"queue_flush": 2048}

    def setUp(self):
        super(TestRunIndexManagementDefault, self).setUp()
        self.idxmgmt = IdxMgmt(self.es, index_name)

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
        self.idxmgmt = IdxMgmt(self.es, index_name)

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
        self.idxmgmt = IdxMgmt(self.es, index_name, [self.custom_policy, default_policy])

    def test_ilm_loaded(self):
        self.idxmgmt.wait_until_created()
        self.idxmgmt.assert_event_template(with_ilm=True)
        self.idxmgmt.assert_alias()
        self.idxmgmt.assert_policies(loaded=True)

        # check out configured policy in apm-server.yml.j2

        # ensure default policy is changed
        policy = self.idxmgmt.fetch_policy(default_policy)
        phases = policy[default_policy]["policy"]["phases"]
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
        self.idxmgmt = IdxMgmt(self.es, index_name)

    def test_template_and_ilm_loaded(self):
        self.idxmgmt.wait_until_created(templates=[index_name], policies=[], aliases=[])
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

        self.idxmgmt = IdxMgmt(self._es, index_name)
        cleanup(self._es, delete_pipelines=[])
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
        self.idxmgmt.assert_policies(loaded=False)

    def test_setup_template_disabled_ilm_enabled(self):
        """
        Test setup --template when template disabled and ilm enabled
        """
        exit_code = self.run_beat(extra_args=["setup", self.cmd,
                                              "-E", "setup.template.enabled=false"])
        assert exit_code == 0
        self.idxmgmt.assert_template(loaded=0)
