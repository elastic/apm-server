#!/bin/bash

/etc/init.d/apm-server start && \
	sleep 2 && \
	stat /var/lib/apm-server/meta.json /var/log/apm-server/apm-server && \
	/etc/init.d/apm-server stop
