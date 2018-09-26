# APM Rally Track

For general information about rally tracks and how to use them, 
check out the [docs](https://esrally.readthedocs.io/en/stable/index.html).
and the [rally](https://github.com/elastic/rally) and [rally-tracks](https://github.com/elastic/rally-tracks) github 
repos.

## Preparing Data Corpora
Data need to be prepared before you can run a race. Base data used in this track are dumped from example Opbeans applications, instrumented with different agents. The dump was created in October 2018. The base data are stored in a s3 bucket and consist of [error](`http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm/error.json.bzip2`), [span](`http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm/span.json.bzip2`) and [transaction](`http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm/transaction.json.bzip2`) data.

For preparing the benchmark tests with default data, run 
```
virtualenv -p python3 rally/.venv
source .venv/bin/activate
pip install -r rally/_tools/requirements.txt
python ./rally/_tools/prepare.py

```

Use `python ./rally/_tools/prepare.py --help` for more information about config options.

If you want to use the benchmark tool with different data, you can also build your own corpora as a base. 
There is a [python script](https://github.com/elastic/apm-server/blob/d7b9d5027dd6a296792aa5179c0eaff8374d62d8/rally/fetch_data.py), you can use for fetching data from an Elasticsearch instance.
In this case you'd also need to change the `document-count` attribute in the `track.json`.


## Starting a Race

Ensure you run the preparation step before, as the race cannot succeed otherwise.
At the moment APM track offers three different challenges. 

### Default challenge
Ingest data of type `error,transactions,spans` in parallel into Elasticsearch.
You can start the race for the default challenge with `esrally --track-path=<local-path-to-apm-track>`.

### Single Event Type challenge
If you want to test only a dedicated event type, you can use the second challenge, by running 
`esrally --track-path=<local-path-to-apm-track> --track-params="event_type:'<event_type>'" --challenge=ingest-event-type`

### Test Query performance
If you want to test query performance, you have to specify the number of days for which indices should be created and 
data ingested. Run `esrally --track-path=<local-path-to-apm-track> --track-params="index_count:<days>" --challenge=query-apm`


If you want to store results to a file, preserve temporary Elasticsearch, use a running Elasticsearch instance, etc. 
please refer to rally's [command_line_reference](https://esrally.readthedocs.io/en/stable/command_line_reference.html#command-line-flags).


## Parameters
When providing `--track-params` you can override following default parameters for the challenges: 

* ingest_percentage: Percentage defining how much of the document corpus should be indexed. Defaults to 100. 
* bulk-size: Number of documents per bulk operation. Defaults to 1_000.
* bulk_indexing_clients: Number of clients running bulk indexing requests.
* event_type: Event type to index. Only used for the `ingest-event-type` challenge.
* index_count: Number of daily indices to create. Only used for the `query-apm` challenge.

Read more about [track-params](https://esrally.readthedocs.io/en/stable/command_line_reference.html#track-params) in general. 

It is recommended to perform the rally task on a different host than where your Elasticsearch instance is running. 
By default rally downloads and starts an Elasticsearch instance.
You can provide an URL to a running Elasticsearch instance instaed, by adding `--pipeline=benchmark-only 
--target-hosts=<ES-host>:<ES-port>`
