#!/bin/bash

set -e

curl -s localhost:5601 > /dev/null && docker-compose down || true

# Overriding the default kibana configuration to automatically install the APM
# Integration and modify the default settings to listen to all interfaces and
# have Expvar enabled for testing.
echo "-> Updating kibana.yml to install the APM Integration by default..."
cp testing/docker/kibana/kibana-apm-preinstalled.yml testing/docker/kibana/kibana.yml
docker-compose up -d

echo "-> Enabling agent monitoring..."
docker-compose exec fleet-server /fleet-server/agent-monitoring.sh
docker-compose restart fleet-server
docker-compose --profile monitoring up -d
