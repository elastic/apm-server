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
        alias_fields = set()
        for f, a in flatmap(yaml.load(self.command_output)["mappings"]["doc"]["properties"]):
            all_fields.add(f)
            if a.get("type") == "alias":
                alias_fields.add(a["path"])

        # fields with special exception, due to mapping type changes, etc
        # no comment means unchanged
        exception_fields = {
            "@timestamp",
            "context.request.url.port",  # field copy to url.port, keyword -> int
            "context.request.url.protocol",  # field copy to url.scheme, drop trailing ":"
            "context.tags",  # field copy, can't alias objects
            "labels",  # target for context.tags copy
            "parent.id",
            "processor.event", "processor.name",
            "sourcemap.bundle_filepath", "sourcemap.service.name", "sourcemap.service.version",
            "span.duration.us", "span.hex_id", "span.id", "span.name", "span.parent", "span.start.us", "span.type",
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
        }

        should_not_be_aliased = alias_fields - all_fields
        self.assertFalse(should_not_be_aliased, json.dumps(sorted(should_not_be_aliased)))

        # check the migration log too
        with open(self._beat_path_join("_meta", "ecs-migration.yml")) as f:
            for m in yaml.load(f):
                if m.get("alias", True):
                    self.assertIn(m["from"], alias_fields)
                elif m.get("copy_to", False):
                    self.assertIn(m["from"], all_fields)
                self.assertIn(m["to"], all_fields)

        # check that all fields are accounted for
        not_aliased = all_fields - alias_fields - exception_fields
        self.assertFalse(not_aliased,
                         "\nall fields ({:d}):\n{}\n\naliased ({:d}):\n{}\n\nunaccounted for ({:d}):\n{}".format(
                             len(all_fields), json.dumps(sorted(all_fields)),
                             len(alias_fields), json.dumps(sorted(alias_fields)),
                             len(not_aliased), json.dumps(sorted(not_aliased)),
                         ))
