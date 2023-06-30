#!/usr/bin/env bash

kibana_host=$1
kibana_user=$2
kibana_pwd=$3
png_output_file=${4:-'out.png'}
table_width_px=${5:-850}

JOB_PARAMS=$(echo "
(
  layout:(
    dimensions:(
      height:400,
      width:${table_width_px}
    ),id:preserve_layout
  ),
  locatorParams:(
    id:DASHBOARD_APP_LOCATOR,
    params:(
      dashboardId:a5bc8390-2f8e-11ed-a369-052d8245fa04,
      preserveSavedFilters:!t,
      timeRange:(
        from:now-30d,
        to:now
      ),
      useHash:!f,
      viewMode:view
    ),
    version:'8.3.2'
  ),
  objectType:dashboard,
  title:app_bench_diff_shifts_slack,
  version:'8.3.2'
)
" | tr -d "[:space:]")


png_url_path=$(curl -XPOST -v -L -u "$kibana_user:$kibana_pwd" -H 'kbn-xsrf: true' --data-urlencode "jobParams=${JOB_PARAMS}" $kibana_host/api/reporting/generate/pngV2 | jq -r '.path')

if [[ "$png_url_path" == "null" ]]
then
  echo "PNG URL path is null"
  exit 1
fi

echo "PNG URL path: $png_url_path"

attempt_counter=0
max_attempts=5
sleep_timeout=10

sleep $sleep_timeout
until $(curl -L --output /dev/null --head --silent --fail -u "$kibana_user:$kibana_pwd" -H 'kbn-xsrf: true' $kibana_host$png_url_path); do
    if [ ${attempt_counter} -eq ${max_attempts} ];then
      echo "Max attempts reached. PNG report timed out."
      exit 1
    fi

    printf 'PNG is not ready yet...\n'
    attempt_counter=$(($attempt_counter+1))
    sleep $sleep_timeout
done

curl -L --silent --output $png_output_file --fail -u "$kibana_user:$kibana_pwd" -H 'kbn-xsrf: true' $kibana_host$png_url_path
