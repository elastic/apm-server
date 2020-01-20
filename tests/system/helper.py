from datetime import datetime, timedelta
import time
from elasticsearch import Elasticsearch, NotFoundError
from beat.beat import TimeoutError

apm = "apm"
apm_prefix = "{}*".format(apm)


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


def cleanup(es, indices, templates, policies, indices_truncate, skip_pipelines=True):
    wait_until_indices_deleted(es, indices)
    wait_until_templates_deleted(es, templates)
    wait_until_indices_truncated(es, indices_truncate)
    wait_until_policies_deleted(es, policies)

    if not skip_pipelines:
        es.ingest.delete_pipeline(id=apm_prefix, ignore=[400, 404])
        wait_until(lambda: apm_prefix not in es.ingest.get_pipeline(ignore=[400, 404]))


def wait_until_indices_deleted(es, indices):
    assert_apm(indices, "indices")
    if es.indices.get(apm_prefix):
        es.indices.delete(index=apm_prefix, ignore=[400, 404])
        for idx in indices:
            wait_until(lambda: not es.indices.exists(idx), name="index {} to be deleted".format(idx))


def wait_until_indices_truncated(es, indices):
    # truncate, don't delete agent configuration index since it's only created when kibana starts up
    for index in indices:
        if es.count(index=index, ignore_unavailable=True)["count"] > 0:
            es.delete_by_query(index, {"query": {"match_all": {}}},
                               ignore_unavailable=True, wait_for_completion=True)
            wait_until(lambda: es.count(index=index, ignore_unavailable=True)["count"] == 0,
                       max_timeout=30, name="acm index {} to be empty".format(index))


def wait_until_templates_deleted(es, templates):
    assert_apm(templates, "templates")
    if es.indices.get_template(name=apm_prefix, ignore=[400, 404]):
        es.indices.delete_template(name=apm_prefix, ignore=[400, 404])
        for t in templates:
            wait_until(lambda: not es.indices.exists_template(t),
                       name="index template {} to be deleted".format(t))


def wait_until_templates(es, templates):
    wait_until(lambda: len(es.indices.get_template(name=templates, ignore=[404])) == len(templates),
               name="expected templates: {}".format(templates))


def wait_until_aliases(es, aliases):
    url = "/_alias/{}".format(",".join(aliases) if aliases else apm_prefix)
    expected = len(aliases)

    def aliases_exist():
        try:
            return len(es.transport.perform_request('GET', url)) == expected
        except NotFoundError:
            return 0 == expected
    wait_until(aliases_exist, max_timeout=2, name="expected aliases: {}".format(aliases))


def wait_until_policies_deleted(es, policies):
    assert_apm(policies, "policies")
    # clean up policy
    url = url_policies(policies)
    try:
        es.transport.perform_request('DELETE', url)
    except NotFoundError:
        return

    def policies_deleted():
        try:
            es.transport.perform_request('GET', url)
            return False
        except NotFoundError:
            return True
    wait_until(policies_deleted, name="policy to be deleted")


def wait_until_policies(es, policies):
    expected = len(policies)

    def policies_exist():
        try:
            return len(es.transport.perform_request('GET',  url_policies(policies))) == expected
        except NotFoundError:
            return 0 == expected
    wait_until(policies_exist, name="expected policies: {}".format(policies))


def url_policies(policies=None):
    return "/_ilm/policy/{}".format(",".join(policies) if policies else apm_prefix)


def wait_until_pipelines_deleted(es, pipelines):
    assert_apm(pipelines, "pipelines")
    es.ingest.delete_pipeline(id=apm_prefix, ignore=[400, 404])
    wait_until(lambda: len(es.ingest.get_pipeline(id=pipelines, ignore=[400, 404])) == 0,
               name="pipelines deleted")


def wait_until_pipelines(es, pipelines):
    expected = len(pipelines)
    if not pipelines:
        pipelines = apm_prefix
        expected = 0
    wait_until(lambda: len(es.ingest.get_pipeline(pipelines, ignore=[404])) == expected,
               name="expect {} pipelines".format(expected))


def assert_apm(names, kind):
    assert all(name.startswith("apm")
               for name in names), "cleanup assumption broken for {} - not all prefixed with apm: {}".format(kind, names)
