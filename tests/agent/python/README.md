# Python agent testing

This is to setup an environment for the manual testing of the [python agent](https://github.com/elastic/apm-agent-python) with the server.


# Setup

There are two possible setups. If you already have Python installed, you can run the following two commands to get started, with an activated virtualenv and requirements installed::

```
python flaskapp/app.py
```

There is also a docker environment which comes with all the requisites. Run the following command to start the environment:

```
make start
```

## Access node endpoint

In both setups the endpoint can be accessed through `localhost:5000`.


### Docker

If you have the docker setup, you need to change the `apm-server.yml` to access connections not only on localhost but also from remote. Change `localhost:5000` to `:5000` and restart the server.

In `app.py` change `SERVERS=["your-host"],` to contain your local IP.


### Blackbox testing

To execute a manual blackbox test you can run `python blackbox.py` inside the `flaskapp/` directory.
This starts a Python and/or a Node app with an agent installed, sends some requests to it and assert the results in Elasticsearch.

You need ElasticSearch and the ApmServer running locally on their default ports, and install the requirements for each app.
Make sure you run the stack and agents on the version you want to test.

You might also want to run Kibana to have a look yourself at the data.

You can check optional arguments in `blackbox/__main__`

