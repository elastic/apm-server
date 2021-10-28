# Tail-based sampling testing script

### Theory of operation
This script uses an instrumented go app to generate a reproducible number of transactions
and validates that the number of captured transactions expected per the sample rate, along with
the number of metrics generated. The script will spin up an instance of apm-server, but requires an existing instance
elasticsearch is running.
### Usage
All parameters are options, with default values.

```
usage: tbs-testing.py [-h] [-e <elasticsearch_host>]
    [-a <username:password>]
    [-s <tail_based_sampling_rate>]
    [-r <retries>]
    [-c <elasticsearch_cloud_id>]
    [-u <elasticsearch_cloud_auth>]

optional arguments:
-h, --help            show this help message and exit

-e <elasticsearch_host>, --elasticsearch-host <elasticsearch_host>
The Elasticsearch host your apm server is sending data.
 Default 127.0.0.1:9200

-a <username:password>, --authentication <username:password>
Auth token for elasticsearch.
Default: "admin:changeme"

-s <tail_based_sampling_rate>, --sample_rate <tail_based_sampling_rate>
The tail-based sampling rate set in the APM server. Expected values: [0,1)
default: 1

-r <retries>, --retries <retries>
Number of retries to find expected transactions in elastic search.
Each retry increases delay time by 5*retry_count seconds.
For example: on the 2nd failure, the 3rd try will occur in 15 seconds.
default: 3

-c <elasticsearch_cloud_id>, --cloud-id <elasticsearch_cloud_id>
Used for apm-server configuration. Overrides --elasticsearch_host parameter
If --cloud-auth isn't specified, 'cloud.auth' will use parameters from --authentication parameter.

-u <elasticsearch_cloud_auth>, --cloud-auth <elasticsearch_cloud_auth>
used for apm-server configuration. This parameter requires --cloud-id, otherwise it will be ignored.  Overrides --authentication for apm-server
```

### caveats

If the script fails to run to completion an apm server may be left running in the background.

