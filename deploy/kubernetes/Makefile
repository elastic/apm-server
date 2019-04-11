BEAT_VERSION?=$(shell head -n 1 ../../docs/version.asciidoc | cut -c 17- )

all: apm-server-kubernetes-daemonset.yaml apm-server-kubernetes-sidecar.yaml

apm-server-kubernetes-%.yaml:
	cat config.yaml secret.yaml > $@
	sed "s/%VERSION%/${BEAT_VERSION}/g" $*.yaml.tmpl >> $@
