from apmserver import integration_test
from apmserver import ElasticTest

import os
import urlparse
from elasticsearch import Elasticsearch

from helper import wait_until


@integration_test
class Test(ElasticTest):
    def config(self):
        cfg = super(Test, self).config()

        # Extract username/password from the URL, and store
        # separately from elasticsearch_host. This is necessary
        # because Beats will use the userinfo from the URL in
        # preference to specified values.
        url = urlparse.urlparse(cfg["elasticsearch_host"])
        if url.username:
            cfg["elasticsearch_username"] = url.username
        if url.password:
            cfg["elasticsearch_password"] = url.password
        bare_netloc = url.netloc.split('@')[-1]
        url = list(url)
        url[1] = bare_netloc

        # Set a password for the built-in apm_system user, and use that for monitoring.
        monitoring_password = "changeme"
        admin_user = os.getenv("ES_SUPERUSER_USER", "admin")
        admin_password = os.getenv("ES_SUPERUSER_PASS", "changeme")
        admin_es = Elasticsearch([self.get_elasticsearch_url(admin_user, admin_password)])
        admin_es.xpack.security.change_password(username="apm_system",
                                                body='{"password":"%s"}' % monitoring_password)
        cfg.update({
            "elasticsearch_host": urlparse.urlunparse(url),
            "monitoring_enabled": "true",
            "monitoring_elasticsearch_username": "apm_system",
            "monitoring_elasticsearch_password": monitoring_password,
        })
        cfg.update(self.config_overrides)
        return cfg

    def test_connect(self):
        goal_log = "Successfully connected to X-Pack Monitoring endpoint"
        wait_until(lambda: self.log_contains(goal_log), name="apm-server connected to monitoring endpoint")
