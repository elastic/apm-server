#!/usr/bin/env bash

if ! which jq > /dev/null; then
  echo "This script needs jq to be installed"
  exit 1
fi

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <url>"
  exit 1
fi

url="$1"

# this scripts waits until Kibana is available
max_retries=5
initial_delay=2
max_delay=32
timeout=60

retries=0
current_delay=$initial_delay
start_time=$(date +%s)

while [ $retries -lt $max_retries ]; do

  response=$(curl -s -w "\n%{http_code}" $url/api/status -m 3)

  body=$(echo "$response" | sed '$d')

  # Extract the status code
  status_code=$(echo "$response" | tail -n1)

  message=""

  if [ "$status_code" -ne "200" ]; then
    message+="URL call to $url/api/status failed with status code $status_code. Response was: "$body""
  elif [ "$response" ]; then
    available=$(echo $response | jq '.status.core.elasticsearch.level == "available"')
    
    if [ available ]; then
      echo "Kibana is available"
      exit 0
    fi

    message+="Elasticsearch is not yet available"
  fi

  if [ ! "$message" ]; then
    exit 0
  fi

  if [ $retries -eq 0 ]; then
    # trigger a restart in case it's not up and running
    echo "Restarting Kibana"
    touch -c "../kibana/config/kibana.dev.yml"
  fi
    
  echo "$message. Retrying with exponential backoff..."
  sleep $current_delay

  retries=$((retries + 1))
  elapsed_time=$(($(date +%s) - start_time))

  if [ $elapsed_time -ge $timeout ]; then
    echo "Timeout reached."
    exit 1
  fi

  current_delay=$((current_delay * 2))
  if [ $current_delay -gt $max_delay ]; then
    current_delay=$max_delay
  fi
done

echo "Max retries reached."
exit 1
