# Python agent testing

This is to setup an environment for the manual testing of the [python agent](https://github.com/elastic/apm-agent-python) in a dummy Flask app and the APM Server.

There are two possible setups. If you already have Python installed, you can run the following to commands to get started, preferably with an activated virtualenv::

```
pip install -r requirements.txt
python -m app run
```

There is also a docker environment which comes with all the requisites. Run the following command to start the environment:

```
make start
```

In both setups the endpoint can be accessed through `localhost:5000`.

The app is configured to send transactions and errors to `localhost:8200`, but you need to run the APM Server and Elasticsearch yourself.
