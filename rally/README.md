# APM Rally Track

For general information about rally tracks and how to use them, 
check out the [rally docs](https://esrally.readthedocs.io/en/stable/index.html)
and the [rally](https://github.com/elastic/rally) and [rally-tracks](https://github.com/elastic/rally-tracks) github 
repos.

## Starting a Race

Currently APM track offers three different challenges. 
If you want to store results to a file, preserve temporary Elasticsearch, use a running Elasticsearch instance, etc. 
please refer to rally's [command_line_reference](https://esrally.readthedocs.io/en/stable/command_line_reference.html#command-line-flags).

### Default challenge
Ingest data of type `error,transactions,spans` in parallel into Elasticsearch.
You can start the race for the default challenge with:

```esrally --track-path=<local-path-to-apm-track>```

It will download and decompress prepared corpora for `error`, `transaction` and `span` events when running the challenge 
for the first time. Data used in this track are dumped from example Opbeans applications, instrumented with different agents.
The dump was created in October 2018 and it is stored in a s3 bucket.

### Single Event Type challenge
If you want to test only a dedicated event type, you can use the second challenge, by running 

```esrally --track-path=<local-path-to-apm-track> --track-params="event_type:'<event_type>'" --challenge=ingest-event-type```

The same test data as for the default challenge are used.

### Test Query performance
For testing query performance it can be important to have data split up into multiple daily indices. A preparation step 
is necessary to bring the data into the expected format.

For preparing the benchmark tests with default data, run 
```
virtualenv -p python3 rally/.venv
source rally/.venv/bin/activate
pip install -r rally/_tools/requirements.txt
python ./rally/_tools/prepare.py

```

By default testdata for 10 days are created. You can change that by applying cmd line options when starting the 
preparation step. Use `python ./rally/_tools/prepare.py --help` for more information about available options.

Base data used for this track are dumped from example Opbeans applications, instrumented with different agents. The dump was 
created in October 2018. The base data are stored in a s3 bucket and consist of [error](`http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm/error_base.json.bzip2`), 
[span](`http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm/span_base.json.bzip2`) and 
[transaction](`http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm/transaction_base.json.bzip2`) data.


After preparing the testdata you can run the rally track.
You can specify the number of days for which data should be ingested. (Note: the number of days cannot be higher than 
the days for which you created data in the preparation step.)

Run `esrally --track-path=<local-path-to-apm-track> --track-params="index_count:<days>" --challenge=query-apm`


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

## Using a different dataset
If you want to use the benchmark tool with different data, you can also build your own corpora as a base. 
There is a [python script](https://github.com/elastic/apm-server/blob/d7b9d5027dd6a296792aa5179c0eaff8374d62d8/rally/fetch_data.py), you can use for fetching data from an Elasticsearch instance.
In this case you'd also need to change the `document-count` attribute in the `track.json`.


