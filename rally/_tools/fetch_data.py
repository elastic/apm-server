from elasticsearch import Elasticsearch
from elasticsearch import helpers
import json
import os


def fetch(path, q, service):
    es = Elasticsearch(["http://localhost:9200"])
    events = {"error": 1000000,
              "transaction": 5000000,
              "span": 20000000}
    batch_size = 1000
    for ev, doc_ct in events.items():
        print("fetching up to {} data for {}".format(doc_ct, ev))
        idx = "apm*{}*".format(ev)
        ct = 0
        name = ""
        if service != "":
            name = "{}_".format(service)
        f = os.path.join(path, "{}{}_base.json".format(name, ev))
        with open(f, 'w') as out:
            for doc in helpers.scan(es, query=q, index=idx, size=batch_size):
                out.write(json.dumps(doc["_source"]))
                out.write("\n")
                ct += 1
                if ct >= doc_ct:
                    break

        os.system("wc -l {}".format(f))
        os.system("stat -f '%z' {}".format(f))


def fetch_per_service(path):
    services = [
        "opbeans-node",
        "opbeans-python",
        "opbeans-ruby",
        "opbeans-rum",
        "opbeans-go",
        "opbeans-java",
    ]
    # fetch data per event type per agent
    for service in services:
        q = {"query": {"match": {"context.service.name": service}}}
        fetch(path, q, service)


def fetch_all(path):
    # fetch data per event type
    q = {"query": {"match_all": {}}}
    fetch(path, q, "")


def main():
    cwd = os.path.dirname(os.path.realpath(__file__))
    path = os.path.join(cwd, "tmp")
    if not os.path.exists(path):
        os.makedirs(path)
    # fetch_per_service(path)
    fetch_all(path)


if __name__ == '__main__':
    main()
