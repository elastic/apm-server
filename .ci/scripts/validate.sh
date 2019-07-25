#!/usr/bin/env bash
set -eo pipefail

if [ -z "${JENKINS_URL}" ] ; then
  JENKINS_URL=http://0.0.0.0:18080
else
  # See https://jenkins.io/doc/book/pipeline/development/#linter
  JENKINS_CRUMB=$(curl --silent "$JENKINS_URL/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)")
fi

## Validate whether the URL is reachable before running anything else
curl --silent ${JENKINS_URL}/ > /dev/null

## Iterate for each file without failing fast.
set +e
for file in "$@"; do
  if curl --silent -X POST -H "${JENKINS_CRUMB}" -F "jenkinsfile=<${file}" ${JENKINS_URL}/pipeline-model-converter/validate | grep -i -v successfully ; then
    echo "ERROR: jenkinslint failed for the file '${file}'"
    exit_status=1
  fi
done

exit $exit_status
