# fleetctl

`fleetctl` provides a minimal CLI for interacting with the Fleet API,
for use in development and testing. This tool may be useful for
manipulating APM integration config vars that are not shown in the
APM integration policy editor.

# Examples

## Listing policies

`fleetctl -u <kibana_url> list-policies`

## Updating vars

`fleetctl -u <kibana_url> set-policy-var <ID> tail_sampling_storage_limit=30MB`

