#!/usr/bin/env python3

import argparse
import json
import tempfile
import shlex
import shutil
import subprocess
import sys
import os


def exec(cmd):
    """
    Executes the given command and returns its result as a string
    """
    try:
        out = subprocess.check_output(shlex.split(cmd))
    except subprocess.CalledProcessError as e:
        print(e)
        sys.exit(1)

    return out.decode("utf-8")


def call(cmd):
    """
    Executes the given command while showing progress in stdout / stderr
    """
    code = subprocess.call(shlex.split(cmd))
    if code > 0:
        sys.exit(code)


def main(branch):
    """
    Updates the index pattern in kibana in the specified branch (default "master")

    NOTES:
    - `make && make update` must have been run previously
    """

    apm_bin = os.path.abspath(os.path.join(os.path.dirname(__file__), "../apm-server"))
    export = exec(apm_bin + " export index-pattern")
    index_pattern = json.loads(export)["objects"][0]

    remote_url = exec("git config remote.origin.url")
    gh_user = remote_url.split(":")[1].split("/")[0]
    print("branch: " + branch)
    print("github user: " + gh_user)

    path = tempfile.mkdtemp()
    print("checking out kibana in temp dir " + path)
    os.chdir(path)
    call("git clone --depth 1 git@github.com:" + gh_user + "/kibana.git .")
    call("git remote add elastic git@github.com:elastic/kibana.git")
    call("git fetch elastic " + branch)
    call("git checkout -b update-apm-index-pattern-" + branch + " elastic/" + branch)
    call("git pull")

    kibana_file_path = "x-pack/plugins/apm/server/tutorial/index_pattern.json"

    with open(kibana_file_path, 'r+') as kibana_file:
        data = json.load(kibana_file)
        old_fields = set([item["name"] for item in json.loads(data["attributes"]["fields"])])
        new_fields = set([item["name"] for item in json.loads(index_pattern["attributes"]["fields"])])
        print("added fields :" + repr(new_fields.difference(old_fields)))
        print("removed fields :" + repr(old_fields.difference(new_fields)))

        del index_pattern["attributes"]["title"]
        kibana_file.seek(0)
        kibana_file.write(json.dumps(index_pattern, indent=2, sort_keys=True))
        kibana_file.truncate()

    call("git add " + kibana_file_path)
    call('git commit -m "update apm index pattern"')
    call("git push --force origin update-apm-index-pattern-" + branch)

    print("removing " + path)
    shutil.rmtree(path)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-b', default='master', dest='branch')
    args = parser.parse_args()
    main(args.branch)
