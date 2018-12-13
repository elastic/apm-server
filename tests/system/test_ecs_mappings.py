import json

import yaml

from apmserver import SubCommandTest


def flatmap(fields, pfx=None):
    if pfx is None:
        pfx = []
    for field, attribs in sorted(fields.items()):
        if 'properties' in attribs:
            for f in flatmap(attribs['properties'], pfx + [field]):
                yield f
        else:
            yield ".".join(pfx + [field]), attribs


class ECSTest(SubCommandTest):
    """
    Test export template subcommand.
    """

    def start_args(self):
        return {
            "extra_args": ["export", "template"],
            "logging_args": None,
        }

    def test_ecs_migration(self):
        """
        Test that all fields are aliased or otherwise accounted for in ECS migration.
        """
        all_fields = set()
        alias_source_fields = set()
        alias_target_fields = set()
        exception_fields = set()
        for f, a in flatmap(yaml.load(self.command_output)["mappings"]["doc"]["properties"]):
            if a.get("type") == "object" and not a.get("enabled", True):
                exception_fields.add(f)
            if not a.get("index", True):
                exception_fields.add(f)
            all_fields.add(f)
            if a.get("type") == "alias":
                alias_source_fields.add(f)
                alias_target_fields.add(a["path"])

        # fields with special exception, due to mapping type changes, etc
        # no comment means unchanged
        exception_fields.update({
            "@timestamp",
            "container.labels",  # target for docker.container.labels copy
            "context.request.url.port",  # field copy to url.port, keyword -> int
            "context.request.url.protocol",  # field copy to url.scheme, drop trailing ":"
            "context.tags",  # field copy, can't alias objects
            "docker.container.labels",  # field copy, can't alias objects
            "error.code", "error.culprit", "error.exception.code", "error.exception.handled", "error.exception.message",
            "error.exception.module", "error.exception.type", "error.grouping_key", "error.id", "error.log.level",
            "error.log.logger_name", "error.log.message", "error.log.param_message", "error.message", "error.type",
            "fields",
            "labels",  # target for context.tags copy
            "kubernetes.annotations", "kubernetes.container.image", "kubernetes.container.name", "kubernetes.labels",
            "kubernetes.namespace", "kubernetes.node.name", "kubernetes.pod.name", "kubernetes.pod.uid",
            "parent.id",
            "process.args",
            "processor.event", "processor.name",
            "sourcemap.bundle_filepath", "sourcemap.service.name", "sourcemap.service.version",
            "span.duration.us", "span.hex_id", "span.id", "span.name", "span.parent", "span.start.us", "span.sync",
            "span.action", "span.subtype", "span.type",
            "system.cpu.total.norm.pct", "system.memory.actual.free", "system.memory.total",
            "system.process.cpu.total.norm.pct", "system.process.memory.rss.bytes", "system.process.memory.size",
            "tags",
            "timestamp.us",
            "trace.id",
            "transaction.duration.us", "transaction.id", "transaction.marks.navigationTiming", "transaction.name",
            "transaction.result", "transaction.sampled", "transaction.span_count.dropped.total", "transaction.type",
            "url.port",  # field copy from context.request.url.port
            "url.scheme",  # field copy from context.request.url.protocol

            # scripted fields
            "error id icon",
            "view errors",
            "view spans",

            # host processor fields, already ECS compliant
            "host.id", "host.mac", "host.name", "host.os.family", "host.os.version"
        })

        # TBD
        exception_fields.update({
            "context.http.status_code", "context.response.finished", "context.user.user-agent",
        })

        should_not_be_aliased = alias_target_fields - all_fields
        self.assertFalse(should_not_be_aliased, json.dumps(sorted(should_not_be_aliased)))

        # check that all fields are accounted for
        not_aliased = all_fields - alias_target_fields - alias_source_fields - exception_fields
        fmt = "\nall fields ({:d}):\n{}\n\naliased ({:d}):\n{}\n\naliases ({:d}):\n{}\n\nunaccounted for ({:d}):\n{}"
        self.assertFalse(not_aliased,
                         fmt.format(
                             len(all_fields), json.dumps(sorted(all_fields)),
                             len(alias_target_fields), json.dumps(sorted(alias_target_fields)),
                             len(alias_source_fields), json.dumps(sorted(alias_source_fields)),
                             len(not_aliased), json.dumps(sorted(not_aliased)),
                         ))

    def test_ecs_migration_log(self):
        aliases = {}
        all_fields = set()
        not_indexed = set()
        for f, a in flatmap(yaml.load(self.command_output)["mappings"]["doc"]["properties"]):
            self.assertNotIn(f, all_fields)
            self.assertNotIn(f, aliases)
            all_fields.add(f)
            if a.get("type") == "alias":
                aliases[f] = a["path"]
            if not a.get("index", True):
                not_indexed.add(f)

        aliases_logged = {}
        for migration_log in self._beat_path_join("_meta", "ecs-migration.yml"), \
                             self._beat_path_join("_beats", "dev-tools", "ecs-migration.yml"):
            with open(migration_log) as f:
                for m in yaml.load(f):
                    if m.get("index", "apm-server") != "apm-server":
                        continue
                    if m.get("alias", True):
                        self.assertNotIn(m["to"], aliases_logged, "duplicate log entry for: {}".format(m["to"]))
                        aliases_logged[m["to"]] = m["from"]
                    elif m.get("copy_to", False):
                        if m["from"] not in not_indexed and m["to"] not in not_indexed:
                            self.assertIn(m["from"], all_fields)
                    else:
                        self.fail(m)
                    self.assertIn(m["to"], all_fields)

        # false if ecs-migration.yml log does not match fields.yml
        self.assertEqual(aliases, aliases_logged)
