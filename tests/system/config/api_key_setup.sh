#!/usr/bin/env bash

set -ex

if [ -z "$2" ]
  then
    exit 1
fi
user="$1"
pwd="$2"

# create privileges in ES for apm_system user
curl -u "$user:$pwd" -XPUT "http://localhost:9200/_security/privilege" -H 'Content-Type: application/json' -d '
{
  "apm-backend": {
    "access": {
      "actions": [ "action:access" ]
    },
    "sourcemap": {
      "actions": [ "action:sourcemap" ]
    },
    "intake": {
      "actions": [ "action:intake" ]
    },
    "config": {
      "actions": [ "action:config" ]
    },
    "full": {
      "actions": [ "action:full" ]
    },
    "elastic-only": {
      "actions": [ "action:elastic-only" ]
    }
  }
}' > privileges_created
