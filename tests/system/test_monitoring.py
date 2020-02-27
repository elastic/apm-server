import os
from urllib.parse import urlparse, urlunparse

from elasticsearch import Elasticsearch
from apmserver import integration_test
from apmserver import ElasticTest
from helper import wait_until


@integration_test
class Test(ElasticTest):
    def config(self):
        cfg = super(Test, self).config()

        # Extract username/password from the URL, and store
        # separately from elasticsearch_host. This is necessary
        # because Beats will use the userinfo from the URL in
        # preference to specified values.
        url = urlparse(cfg["elasticsearch_host"])
        if url.username:
            cfg["elasticsearch_username"] = url.username
        if url.password:
            cfg["elasticsearch_password"] = url.password
        bare_netloc = url.netloc.split('@')[-1]
        url = list(url)
        url[1] = bare_netloc

        # Set a password for the built-in apm_system user, and use that for monitoring.
        monitoring_password = "changeme"
        self.admin_es.xpack.security.change_password(username="apm_system",
                                                     body='{"password":"%s"}' % monitoring_password)
        cfg.update({
            "elasticsearch_host": urlunparse(url),
            "monitoring_enabled": "true",
            "monitoring_elasticsearch_username": "apm_system",
            "monitoring_elasticsearch_password": monitoring_password,
        })
        cfg.update(self.config_overrides)
        return cfg

    def test_connect(self):
        goal_log = "Successfully connected to X-Pack Monitoring endpoint"
        wait_until(lambda: self.log_contains(goal_log), name="apm-server connected to monitoring endpoint")
