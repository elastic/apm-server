#!/


import subprocess
import requests
import json
import argparse

from time import sleep
from datetime import datetime, timezone
from requests.auth import HTTPBasicAuth

metric_query = {
    "aggs": {
      "count": {
          "value_count": {
              "field": "transaction.duration.histogram"
          }
      }
    },
    "query": {
        "bool": {
            "filter": [
                {
                    "term": {
                        "transaction.type": "request"
                    }
                },
                {
                    "term": {
                        "service.name": "goagent"
                    }
                },
                {
                    "range": {
                        "@timestamp": {
                            "gte": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ"),
                            "lte": "now"
                        }
                    }
                }
            ]
        }
    },
    "size": 0
}

trace_query = {
    "query": {
        "bool": {
            "filter": [
                {
                    "term": {
                        "service.name": "goagent"
                    }
                },
                {
                    "term": {
                        "transaction.type": "request"
                    }
                },
                {
                    "range": {
                        "@timestamp": {
                            "gte": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ"),
                            "lte": "now"
                        }
                    }
                }
            ]
        }
    },
    "size": 0
}


def metric_search(host, user, password):
    response = requests.get(host + "/metrics*/_search",
                            auth=HTTPBasicAuth(user, password),
                            data=json.dumps(metric_query),
                            headers={"content-type": "application/json; charset=utf8"})
    return response.json()["aggregations"]["count"]["value"]


def trace_search(host, user, password):
    response = requests.get(host + "/traces*/_search",
                            auth=HTTPBasicAuth(user, password),
                            data=json.dumps(trace_query),
                            headers={"content-type": "application/json; charset=utf8"})
    return response.json()["hits"]["total"]["value"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-e",
                        "--elasticsearch_host",
                        required=False,
                        type=str,
                        default="localhost:8200",
                        dest="host",
                        metavar="<elasticsearch_host>",
                        help="the Elasticsearch host your apm server is sending data")
    parser.add_argument("-a",
                        "--authentication",
                        required=False,
                        type=str,
                        default="admin:changeme",
                        dest="auth",
                        metavar="<username:password>",
                        help="auth token for elasticsearch")
    parser.add_argument("-s",
                        "--sample_rate",
                        required=False,
                        type=float,
                        default=1,
                        dest="sample_rate",
                        metavar="<tail_based_sampling_rate>",
                        help="the tail-based sampling rate set in the APM server.")
    parser.add_argument("-r",
                        "--retries",
                        required=False,
                        type=int,
                        default=3,
                        dest="retry",
                        metavar="<retries>",
                        help="how many retries to find expected transactions in elastic search.")
    parser.add_argument("-c",
                        "--cloud-id",
                        required=False,
                        type=str,
                        default="",
                        dest="cloud_id",
                        metavar="<elasticsearch_cloud_id>",
                        help="used for apm-server configuration.")
    parser.add_argument("-u",
                        "--cloud-auth",
                        required=False,
                        type=str,
                        default="",
                        dest="cloud_auth",
                        metavar="<elasticsearch_cloud_auth>",
                        help="used for apm-server configuration. Overrides --authentication for apm-server")

    args = parser.parse_args()

    elastic_host = args.host

    cloud_id = args.cloud_id

    cloud_auth = args.auth

    if len(args.cloud_auth) > 0:
        cloud_auth = args.cloud_auth
    auth = args.auth.split(':')
    elastic_user = auth[0]
    elastic_password = auth[1]

    sample_rate = args.sample_rate

    retry = args.retry
    if len(cloud_id) > 0:
        es_yml = f"""cloud.id: "{cloud_id}"
cloud.auth: "{cloud_auth}
        """
    else:
        es_yml = f"""output.elasticsearch:
    hosts: [{elastic_host}]
    username: {elastic_user}
    password: {elastic_password}
"""

    apm_yml = f"""apm-server:
  host: "0.0.0.0:8200"
  data_streams.enabled: true
  sampling:
    tail:
      enable: true
      keep_unsampled: false
      policies: [ {{ sample_rate: {sample_rate} }} ]
      ttl: 30m
{es_yml}
"""

    f = open("tmp.yml", "w")
    f.write(apm_yml)
    f.close()

    #todo: how to find apm-server binary?
    server_p = subprocess.Popen("../../apm-server -c ./tmp.yml -e &", stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, shell=True)

    cmd = "go run ."
    p = subprocess.run(cmd.split(), stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    results = 0
    for i in p.stdout.decode("utf-8").split('\n'):
        for e in i.split():
            kvp = e.split(':')
            if kvp[0] == "TransactionsSent":
                results += int(kvp[1])
                break

    for i in range(1, retry + 1):
        metric_result = metric_search(elastic_host, elastic_user, elastic_password)
        trace_result = trace_search(elastic_host, elastic_user, elastic_password)
        if results * sample_rate == trace_result:
            print("sent transactions: " + str(results))
            print("sample rate: " + str(sample_rate))
            print("expected transactions: " + str(results * sample_rate))
            print("found transactions: " + str(trace_result))
            print("found metrics: " + str(metric_result))
            print("success!")
            subprocess.run(f"kill -9 {server_p.pid}", shell=True)
            exit(0)
        else:
            print("sent transactions: " + str(results))
            print("sample rate: " + str(sample_rate))
            print("expected transactions: " + str(results * sample_rate))
            print("found transactions: " + str(trace_result))
            print("found metrics: " + str(metric_result))
            print("retrying in " + str(i * 5) + " seconds...")
            sleep(i * 5)
            print("retrying...\n\n")

    print("Error: failed to confirm transaction count.")
    subprocess.run(f"kill -9 {server_p.pid}", shell=True)
    exit(1)


if __name__ == '__main__':
    main()
