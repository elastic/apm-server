#!/bin/sh

umask 0027
exec /usr/share/apm-server/bin/apm-server \
  --path.home /usr/share/apm-server \
  --path.config /etc/apm-server \
  --path.data /var/lib/apm-server \
  --path.logs /var/log/apm-server \
  "$@"
