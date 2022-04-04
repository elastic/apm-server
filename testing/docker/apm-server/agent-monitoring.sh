#!/bin/bash

AGENT_CONFIG=/usr/share/elastic-agent/state/fleet.yml

LINE_N="$(grep -n 'enabled: false' ${AGENT_CONFIG}|cut -d ':' -f1)s"
sed -i "${LINE_N}/.*/    enabled: true/" ${AGENT_CONFIG}
LINE_N="$(grep -n 'host: ""' ${AGENT_CONFIG}|cut -d ':' -f1)s"
sed -i "${LINE_N}/.*/    host: 0.0.0.0/" ${AGENT_CONFIG}
