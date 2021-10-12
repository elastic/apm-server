#!/


import subprocess
import requests
import json
import argparse

from time import sleep
from datetime import datetime, timezone
from requests.auth import HTTPBasicAuth

trace_search = {
    "query": {
        "bool": {
            "filter": [
                {
                    "term": {
                        "service.name": "apmbench"
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


def es_search(host, user, password):
    response = requests.get(host + "/traces*/_search", auth=HTTPBasicAuth(user, password),
                            data=json.dumps(trace_search), headers={"content-type": "application/json; charset=utf8"})
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

    args = parser.parse_args()

    elastic_host = args.host

    auth = args.auth.split(':')
    elastic_user = auth[0]
    elastic_password = auth[1]

    sample_rate = args.sample_rate

    retry = args.retry

    cmd = "go run ."
    p = subprocess.run(cmd.split(), stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    bench_result = p.stderr.decode("utf-8").split()
    print(p.stdout.decode("utf-8"))
    iterations = bench_result[1]

    print("es query : \n" + json.dumps(trace_search, indent=2) + "\n\n")

    for i in range(1, retry + 1):
        trace_result = es_search(elastic_host, elastic_user, elastic_password)
        if iterations * sample_rate == trace_result:
            exit(0)
        else:
            print("sent transactions: " + str(iterations))
            print("sample rate: " + str(sample_rate))
            print("expected transactions: " + str(iterations * sample_rate))
            print("found transactions: " + str(trace_result))
            print("retrying in " + str(i * 5) + " seconds...")
            sleep(i * 5)
            print("retrying...\n\n")

    print("Error: failed to confirm transaction count.")
    exit(1)


if __name__ == '__main__':
    main()
