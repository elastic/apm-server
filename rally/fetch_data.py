from elasticsearch import Elasticsearch
from elasticsearch import helpers
import json
import os


def fetch(q, service):
    es = Elasticsearch(["http://localhost:9200"])
    events = ["error", "transaction", "span"]
    doc_ct = 1000000
    batch_size = 1000
    for et in events:
        idx = "apm*{}*".format(et)
        ct = 0
        f = os.path.join("rally", "documents")
        name = ""
        if service != "":
            name = "{}_".format(service)
        f = os.path.join(f, "{}{}.json".format(name, et))
        with open(f, 'a+') as out:
            for doc in helpers.scan(es, query=q, index=idx, size=batch_size):
                out.write(json.dumps(doc["_source"]))
                out.write("\n")

                ct += 1
                if ct >= doc_ct:
                    break

        os.system("wc -l {}".format(f))
        os.system("stat -f '%z' {}".format(f))


def main():
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
        fetch(q, service)

    # fetch data per event type
    q = {"query": {"match_all": {}}}
    fetch(q, "")


if __name__ == '__main__':
    main()
