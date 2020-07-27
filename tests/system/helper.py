from datetime import datetime, timedelta
import os
import time
from beat.beat import TimeoutError


def get_version():
    """
    parse the version out of version/version.go
    """
    version_go = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "version", "version.go"))
    match = "const defaultBeatVersion = \""
    with open(version_go) as f:
        for line in f:
            if line.startswith(match):
                return line[len(match):-2]
    raise Exception("version not found")


def wait_until(cond, max_timeout=10, poll_interval=0.25, name="cond"):
    """
    Like beat.beat.wait_until but catches exceptions
    In a ElasticTest `cond` will usually be a query, and we need to keep retrying
     eg. on 503 response codes
    """
    assert callable(cond), "First argument of wait_until must be a function"

    start = datetime.now()
    while datetime.now()-start < timedelta(seconds=max_timeout):
        try:
            result = cond()
            if result:
                return result
        except AttributeError as ex:
            raise ex
        except Exception as ex:
            pass
        time.sleep(poll_interval)
    raise TimeoutError("Timeout waiting for '{}' to be true. ".format(name) +
                       "Waited {} seconds.".format(max_timeout))
