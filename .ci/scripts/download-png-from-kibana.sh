#!/usr/bin/env bash

kibana_host=$1
kibana_user=$2
kibana_pwd=$3
png_output_file=${4:-'out.png'}
table_width_px=${5:-850}

png_url_path=$(curl -XPOST --silent -L -u "$kibana_user:$kibana_pwd" -H 'kbn-xsrf: true' "$kibana_host/api/reporting/generate/pngV2?jobParams=%28browserTimezone%3AEurope%2FBerlin%2Clayout%3A%28dimensions%3A%28height%3A0%2Cwidth%3A$table_width_px%29%2Cid%3Apreserve_layout%29%2ClocatorParams%3A%28id%3ADASHBOARD_APP_LOCATOR%2Cparams%3A%28dashboardId%3Aa5bc8390-2f8e-11ed-a369-052d8245fa04%2CpreserveSavedFilters%3A%21t%2CtimeRange%3A%28from%3Anow-30d%2Cto%3Anow%29%2CuseHash%3A%21f%2CviewMode%3Aview%29%2Cversion%3A%278.3.2%27%29%2CobjectType%3Adashboard%2Ctitle%3Aapp_bench_diff_shifts_slack%2Cversion%3A%278.3.2%27%29" \
| jq -r '.path')

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