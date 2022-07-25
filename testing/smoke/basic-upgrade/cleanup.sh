#!/bin/bash

set -eo pipefail

# Loading smoke tests lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

# Run terraform destroy to clean underlying infra
terraform_destroy
