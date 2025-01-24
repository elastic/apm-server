#!/usr/bin/env bash

if ! getent group apm-server >/dev/null; then
  groupadd -r apm-server
fi

if ! getent passwd apm-server >/dev/null; then
  useradd -r -g apm-server -d /var/lib/apm-server -c "apm-server" apm-server
fi
