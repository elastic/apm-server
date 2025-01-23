# An opinionated approach to manage the Elatic Qualifier for the DRA in a Google Bucket
# Instead of using the ELASTIC_QUALIFIER env variable.
fetch_elastic_qualifier() {
  local branch=$1
  qualifier=""
  if curl -sf -o /dev/null "https://storage.googleapis.com/artifacts-api/test/$branch" ; then
    qualifier=$(curl -s "https://storage.googleapis.com/artifacts-api/test/$branch")
  fi
  echo "$qualifier"
}

retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** count))
        count=$((count + 1))
        if [ $count -lt "$retries" ]; then
            >&2 echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            >&2 echo "Retry $count/$retries exited $exit, no more retries left."
            return $exit
        fi
    done
    return 0
}
