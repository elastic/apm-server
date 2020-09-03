from elasticsearch import NotFoundError, RequestError
from helper import wait_until
import time

apm = "apm"
apm_prefix = "{}*".format(apm)
apm_version = "7.9.2"
day = time.strftime("%Y.%m.%d", time.gmtime())
default_policy = "apm-rollover-30-days"
policy_url = "/_ilm/policy/"
default_pipelines = ["apm_user_agent", "apm_user_geo", "apm"]
ilm_pattern = "-000001"
index_name = "apm-{}".format(apm_version)
index_onboarding = "apm-{}-onboarding-{}".format(apm_version, day)
index_smap = "apm-{}-sourcemap".format(apm_version)
index_error = "apm-{}-error".format(apm_version)
index_transaction = "apm-{}-transaction".format(apm_version)
index_span = "apm-{}-span".format(apm_version)
index_metric = "apm-{}-metric".format(apm_version)
index_profile = "apm-{}-profile".format(apm_version)
default_aliases = [index_error, index_transaction, index_span, index_metric, index_profile]
default_indices = [index_name, index_onboarding, index_smap] + default_aliases


def cleanup(es, delete_indices=[apm_prefix], delete_templates=[apm_prefix],
            delete_policies=[default_policy], delete_pipelines=[default_pipelines]):
    wait_until_indices_deleted(es, delete_indices=delete_indices)
    wait_until_templates_deleted(es, delete_templates=delete_templates)
    wait_until_policies_deleted(es, delete_policies=delete_policies)
    wait_until_pipelines_deleted(es, delete_pipelines=delete_pipelines)


# some gotchas when deleting indices and templates:
#
# `es.indices.delete(index=[a,b],ignore=[404,400])` would not throw an exception, but stop running the delete cmd on the ES side as soon as the first 400 or 404 occurs,
# therefore it is important to run one delete command per index
# `es.indices.get("apm*")` always returns `True` because of the `*`,
# bypass this by fetching all indices separately and ensuring none can be found

def wait_until_indices_deleted(es, delete_indices=[apm_prefix]):
    # avoid unnecessary delete requests to ES if possible
    if not delete_indices or (all_apm(delete_indices) and len(es.indices.get(apm_prefix)) == 0):
        return

    for idx in delete_indices:
        es.indices.delete(idx, ignore=[404, 400])

    def is_deleted(idx):
        try:
            return len(es.indices.get(idx)) == 0
        except:
            return True

    for idx in delete_indices:
        wait_until(lambda: is_deleted(idx), name="index {} to be deleted".format(idx))


def wait_until_templates_deleted(es, delete_templates=[apm_prefix]):
    # avoid unnecessary delete requests to ES if possible
    if not delete_templates or (all_apm(delete_templates) and len(es.indices.get_template(apm_prefix, ignore=[404])) == 0):
        return

    for template in delete_templates:
        es.indices.delete_template(template, ignore=[404, 400])
    wait_until_templates(es, templates=delete_templates, exist=False)


def wait_until_policies_deleted(es, delete_policies=[default_policy]):
    if not delete_policies:
        return
    for policy in delete_policies:
        url = "{}{}".format(policy_url, policy)
        try:
            es.transport.perform_request('DELETE', url)
        except NotFoundError:
            return
    wait_until_policies(es, policies=delete_policies, exist=False)


def wait_until_pipelines_deleted(es, delete_pipelines=default_pipelines):
    if not delete_pipelines:
        return
    for pipeline in delete_pipelines:
        es.ingest.delete_pipeline(id=pipeline, ignore=[404])
    wait_until_pipelines(es, pipelines=delete_pipelines, exist=False)


def wait_until_aliases(es, aliases=[], exist=True):
    url = "/_alias/{}".format(",".join(aliases) if aliases else apm_prefix)
    expected = len(aliases) if exist else 0

    def aliases_exist():
        try:
            return len(es.transport.perform_request('GET', url)) == expected
        except NotFoundError:
            return expected == 0
    wait_until(aliases_exist, name="expected aliases: {}".format(aliases))


def wait_until_templates(es, templates=[], exist=True):
    expected = len(templates) if exist else 0

    def expected_templates():
        try:
            return len(es.indices.get_template(templates if templates else apm_prefix)) == expected
        except:
            return expected == 0
    wait_until(expected_templates, name="expected templates: {}".format(templates))


def wait_until_policies(es, policies=[], exist=True):
    expected = len(policies) if exist else 0
    url = "{}{}".format(policy_url, ",".join(policies if policies else [default_policy]))

    def expected_policies():
        try:
            return len(es.transport.perform_request('GET', url)) == expected
        except NotFoundError:
            return expected == 0
    wait_until(expected_policies, name="expected policies: {}".format(policies))


def wait_until_pipelines(es, pipelines=default_pipelines, exist=True):
    expected = len(pipelines) if exist else 0

    def expected_pipelines():
        try:
            return len(es.ingest.get_pipeline(pipelines if pipelines else default_pipelines)) == expected
        except:
            return expected == 0
    wait_until(expected_pipelines, name="expected pipelines {}".format(pipelines))


def all_apm(names):
    return all(name.startswith("apm") for name in names)
