#!/bin/python


import subprocess
import requests
import json
from requests.auth import HTTPBasicAuth

elastic_host = "https://tbs-7-16.es.eastus2.staging.azure.foundit.no:9243"
elastic_user = "elastic"
elastic_password = "GsoqmlktIXgqlKRk0OFBhAJF"

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
                }
            ]
        }
    },
    "size": 0
}


cmd = "go run ."
# p = subprocess.Popen(cmd.split(), stdout=subprocess.PIPE)
# p.wait()

response = requests.get(elastic_host + "/traces*/_search", auth=HTTPBasicAuth(elastic_user, elastic_password),
                        data=json.dumps(trace_search), headers={"content-type": "application/json; charset=utf8"})

print(response.json())
