#!/bin/bash
#
# This script reads the APM Secret Token from the Elastic Agent policy and stores it
# in a local file to be used as Terraform output from this module.

KIBANA_ENDPOINT=https://0944d03c888748e49d689fa41f653eae.us-west2.gcp.elastic-cloud.com:443/api/fleet/package_policies/elastic-cloud-apm
KIBANA_AUTH=elastic:3G0p0N5zukcI7PH3IEBu8KJC

curl -s -u ${KIBANA_AUTH} ${KIBANA_ENDPOINT} ${KIBANA_ENDPOINT} \
  | jq -r '.item | select(.inputs[].policy_template == "apmserver") .inputs[].vars.secret_token.value' \
  | uniq \
  > "/Users/ericyap/Repos/elastic/apm/apm-server/integrationservertest/tf-TestUpgrade_9_4_0-SNAPSHOT_to_9_5_0-SNAPSHOT_Default/secret_token_value.json"
