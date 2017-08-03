# Python agent testing

This is to setup an environment for the manual testing of the [python agent](https://github.com/opbeat/opbeat_python) with the server.


# Setup

There are two possible setups. If you already have Python installed, you can run the following to commands to get started, preferably with an activated virtualenv::

```
make setup
python app.py
```

There is also a docker environment which comes with all the requisits. Run the follwing command to start the environment:

```
make start
```

## Access node endpoint

In both setups the endpoint can be accessed through `localhost:5000`.


## Running the server

As the python agent still outputs to the old paths, run `apm-server -debug -e -d "*"` so a path is opened also for `/`. Like this you see which requests come in. As soon as the python agent is adapted, this should not be needed anymore.


### Docker

If you have the docker setup, you need to change the `apm-server.yml` to access connections not only on localhost but also from remote. Change `localhost:5000` to `:5000` and restart the server.

In `app.py` change `SERVERS=["your-host"],` to contain your local IP.
