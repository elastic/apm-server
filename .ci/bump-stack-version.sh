#!/usr/bin/env bash
#
# Given the stack version this script will bump the version.
#
# This script is executed by the automation we are putting in place
# and it requires the git add/commit commands.
#
# Parameters:
#	$1 -> the version to be bumped. Mandatory.
#	$2 -> whether to create a branch where to commit the changes to.
#		  this is required when reusing an existing Pull Request.
#		  Optional. Default true.
#
set -euo pipefail
MSG="parameter missing."
VERSION=${1:?$MSG}
CREATE_BRANCH=${2:-true}

OS=$(uname -s| tr '[:upper:]' '[:lower:]')

if [ "${OS}" == "darwin" ] ; then
	SED="sed -i .bck"
else
	SED="sed -i"
fi

echo "Update stack with version ${VERSION}"
${SED} -E -e "s#(image: docker\.elastic\.co/.*):[0-9]+\.[0-9]+\.[0-9]+(-[a-f0-9]{8})?#\1:${VERSION}#g" docker-compose.yml

echo "Commit changes"
if [ "$CREATE_BRANCH" = "true" ]; then
	base=$(git rev-parse --abbrev-ref HEAD | sed 's#/#-#g')
	git checkout -b "update-stack-version-$(date "+%Y%m%d%H%M%S")-${branch}"
else
	echo "Branch creation disabled."
fi
git add docker-compose.yml
git diff --staged --quiet || git commit -m "[Automation] Update elastic stack version to ${VERSION} for testing"
git --no-pager log -1

echo "You can now push and create a Pull Request"
