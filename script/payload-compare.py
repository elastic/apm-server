import random
import json
import zlib
from uuid import uuid4
from collections import defaultdict


NUM_ENDPOINTS = 10
SAMPLE_RATE = 0.1
TRACES_PER_TRANSACTION = 30
NUM_TRANSACTIONS = 10000

print "{:25} {}".format("NUM_ENDPOINTS:", NUM_ENDPOINTS)
print "{:25} {}".format("SAMPLE_RATE:", SAMPLE_RATE)
print "{:25} {}".format("TRACES_PER_TRANSACTION:", TRACES_PER_TRANSACTION)
print "{:25} {}".format("NUM_TRANSACTIONS:", NUM_TRANSACTIONS)
# print "SAMPLE_RATE:", SAMPLE_RATE
# print "TRACES_PER_TRANSACTION:", TRACES_PER_TRANSACTION
# print "NUM_TRANSACTIONS:", NUM_TRANSACTIONS

# SHOULD_GROUP = False
# TRACE_UUIDS = False
# TRANSACTION_UUIDS = True


MATRIX_RUNS = [
    {"SHOULD_GROUP": False, "TRACE_UUIDS": False, "TRANSACTION_UUIDS": False},
    {"SHOULD_GROUP": True, "TRACE_UUIDS": False, "TRANSACTION_UUIDS": False},

    {"SHOULD_GROUP": False, "TRACE_UUIDS": True, "TRANSACTION_UUIDS": False},
    {"SHOULD_GROUP": False, "TRACE_UUIDS": False, "TRANSACTION_UUIDS": False},

    {"SHOULD_GROUP": False, "TRACE_UUIDS": False, "TRANSACTION_UUIDS": True},
    {"SHOULD_GROUP": False, "TRACE_UUIDS": False, "TRANSACTION_UUIDS": False},
]


shop = ["shop", "store", "butik", "sklep", "hranut"]
foot = ["shoe", "foot", "fod", "stopa", "regel"]
filewords = shop + foot
methods = ["GET", "POST", "PUT"]
endpoints = [
    "/api/{}/{}".format(random.choice(shop), random.choice(foot))
    for _ in range(NUM_ENDPOINTS)
]


def sizeof_fmt(num, suffix='B'):
    for unit in ['', 'Ki', 'Mi', 'Gi', 'Ti', 'Pi', 'Ei', 'Zi']:
        if abs(num) < 1024.0:
            return "%3.1f%s%s" % (num, unit, suffix)
        num /= 1024.0
    return "%.1f%s%s" % (num, 'Yi', suffix)


def gen_transaction(TRANSACTION_UUIDS, TRACE_UUIDS):
    if random.random() < SAMPLE_RATE:
        transaction = {
            "name": random.choice(endpoints),
            "type": "request",
            "duration": 251.1,
            "timestamp": "2017-05-09T10:23:{}Z".format(random.randint(0, 60)),
            "context": {
                "user": {
                    "email": "ron@opbeat.com"
                },
                "request": {
                    "headers": {
                        "User-Agent": "Mozilla Chrome Edge",
                        "Cookies": "c1=v1;c2=v2"
                    },
                    "path": "/api/v9/1",
                    "method": "POST"
                },
                "response": {
                    "size": 9232,
                    "headers": {
                        "Content-Type": "application/json"
                    }
                }
            },
            "traces": [gen_trace(TRACE_UUIDS) for _ in range(TRACES_PER_TRANSACTION)]
        }
        if TRANSACTION_UUIDS:
            transaction['id'] = str(uuid4())
        return transaction

    return {
        "name": random.choice(endpoints),
        "type": "request",
        "duration": 251.1,
        "timestamp": "2017-05-09T10:23:{:02}Z".format(random.randint(0, 60)),
    }


def gen_trace(TRACE_UUIDS):
    trace = {
        "name": "{} /{}/{}".format(random.choice(methods), random.choice(shop), random.choice(foot)),
        "type": "http",
        "start": 25.2,
        "end": 40.1,
        "parent": str(uuid4()) if TRACE_UUIDS else random.randint(0, TRACES_PER_TRANSACTION),
        "context": {
            "request": {
                "path": "/{}/{}".format(random.choice(methods), random.choice(shop), random.choice(foot)),
                "host": "internal-backendservice.com",
                "port": 80,
                "query": "q1=v1&q2=v2",
                "headers": {
                    "Accept": "application/json"
                },
            },
            "response": {
                "headers": {
                    "Content-Type": "application/json"
                },
                "size": random.randint(100, 100000)
            },
            "stacktrace": [
                {"filename": "/".join([random.choice(filewords)
                                       for _ in range(random.randint(1, 5))]), "lineno": random.randint(1, 10000)}
                for _1 in range(random.randint(3, 20))
            ],
        }
    }
    if TRACE_UUIDS:
        trace['id'] = str(uuid4())

    return trace


def run(SHOULD_GROUP, TRACE_UUIDS, TRANSACTION_UUIDS):
    transactions = [gen_transaction(TRANSACTION_UUIDS, TRACE_UUIDS) for _ in range(NUM_TRANSACTIONS)]

    if SHOULD_GROUP:
        groups = defaultdict(list)
        for tx in transactions:
            name = tx['name']
            del tx['name']
            del tx['type']
            groups[name].append(tx)

        transactions = []
        for name, group in groups.items():
            transactions.append(
                {
                    "name": name,
                    "type": "http",
                    "timestamp": "2017-05-09T10:23:{:02}Z".format(random.randint(0, 60)),
                    "durations": group
                }
            )

    payload = {
        "app_id": "my-app",
        "agent": "opbeat-node/4.1.4",
        "platform": "lang=python/2.7.1 platform=CPython framework=Django/1.11.1",
        "transactions": transactions
    }
    return json.dumps(payload)


for args in MATRIX_RUNS:
    payload = run(**args)
    size = len(payload)
    compressed = len(zlib.compress(payload))

    print " ".join(["{:<25}".format("{}: {}".format(k, v)) for k, v in args.items()]) + ":", sizeof_fmt(compressed), "(uncompressed: {})".format(sizeof_fmt(size))
