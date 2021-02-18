#!/bin/sh

AUTOPEP8FLAGS=--max-line-length=120

exec find -name '*.py' -and -not -path '*build/*' -exec autopep8 $AUTOPEP8FLAGS $* '{}' +
