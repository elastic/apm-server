#!/usr/bin/env python

from __future__ import print_function

import argparse

import requests
import os
import json
import jsondiff
import sys


def fetch_kibana_file(repo_path, ref, file_name):
    path = "{}/{}/{}".format(repo_path, ref, file_name)
    print("---- Comparing Index Pattern with " + path)
    rsp = requests.get(path)
    if rsp.status_code != 200:
        print("failed to query '{}'".format(path))
        return 1
    return rsp.json()


def json_val(v1, v2):
    try:
        return (json.loads(v1), json.loads(v2))
    except:
        return (v1, v2)


def find_key(item):
    if "id" in item:
        return "id"
    elif "name" in item:
        return "name"
    elif "type" in item:
        return "type"
    elif "query" in item:
        return "query"
    elif "value" in item:
        return "value"
    else:
        return ""


def find_item(inp, key, val):
    for entry in inp:
        if not isinstance(entry, dict):
            return ""
        if key in entry and entry[key] == val:
            return entry
    return ""


def build_key(k1, k2):
    if k1 == "":
        return k2
    if k2 == "":
        return k1
    return "{}.{}".format(k1, k2)


def iterate(val_id, key, v1, v2):
    ret_val = 0
    if isinstance(v1, dict) and isinstance(v2, dict):
        for k, v in v1.items():
            ret_val = max(ret_val, iterate(val_id, build_key(key, k), *json_val(v, v2[k] if k in v2 else "")))
    elif isinstance(v1, list) and isinstance(v2, list):
        v1, v2 = json_val(v1, v2)
        # assumption: an array only contains items of same type
        if len(v1) > 0 and isinstance(v1[0], dict):
            for item in v1:
                qkey = find_key(item)
                if qkey == "":
                    print("Script is missing type to compare {}".format(item))
                    return 3

                item2 = find_item(v2, qkey, item[qkey])
                ret_val = max(ret_val, iterate(val_id, build_key(key, "{}={}".format(qkey, item[qkey])), item, item2))
        else:
            v1, v2 = sorted(v1), sorted(v2)
            for item1, item2 in zip(v1, v2):
                ret_val = max(ret_val, iterate(val_id, key, *json_val(item1, item2)))
    else:
        d = jsondiff.JsonDiffer(syntax='symmetric').diff(*json_val(v1, v2))
        if d:
            if key == "attributes.title" or key == "attributes.fields.name=transaction.marks.*.*":
                return ret_val
            ret_val = 2
            print("Difference for id '{}' for key '{}'".format(val_id, key))
            try:
                print(json.dumps(d, indent=4))
            except:
                print(d)
            print("Value in APM Server: {}".format(v1))
            print("Value in Kibana: {}".format(v2))
            print("---")
    return ret_val


def get_kibana_commit(branch):
    """
    Looks up an open PR in Kibana against `branch`, and with 'apm', 'update', and 'index pattern' in the title (case insensitive).
    If found, it is assumed to be a PR updating the Kibana index pattern - so this tests compares the content against
    the one in that PR
    Limitations:
        - `index_pattern.json` must be found in HEAD (so in case of being amended, it needs to be force-pushed)
        - returns the last PR open against a given branch, which might be wrong if there are several updated at a time.
    """
    rsp = requests.get("https://api.github.com/repos/elastic/kibana/pulls")
    if rsp.status_code == 200:
        for pr in rsp.json():
            matches_branch = pr['base']['ref'] == branch
            matches_index_pattern_update = all(
                token in pr['title'].lower() for token in ['apm', 'update', 'index pattern'])
            if matches_branch and matches_index_pattern_update:
                return pr['head']['sha']
    return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('branch')
    parser.add_argument('-I', '--index-pattern', type=str,
                        default='src/legacy/core_plugins/kibana/server/tutorials/apm/saved_objects/index_pattern.json',
                        help='index-pattern file path')
    parser.add_argument('-P', '--repo-path', type=str,
                        default='https://raw.githubusercontent.com/elastic/kibana',
                        help='base repository path')
    parser.add_argument('-C', '--commit', type=str, help='Commit sha to get the index-pattern from')
    args = parser.parse_args()
    exit_val = 0

    ref = args.commit
    if ref is None:
        ref = get_kibana_commit(args.branch)
    if ref is None:
        ref = args.branch

    k = fetch_kibana_file(args.repo_path, ref, args.index_pattern)
    if k == 1:
        print("Kibana file containing index pattern not found")
        return 1

    with open(os.path.abspath(os.path.join('index-pattern.json'))) as f:
        s = json.load(f)["objects"][0]
    exit_val = max(exit_val, iterate(k["id"], "", s, k))
    if exit_val == 0:
        print("up-to-date")
    if "title" in k["attributes"]:
        print("`title` need to be set dynamically, remove it from the index-pattern!")
        exit_val = 3

    return exit_val


if __name__ == '__main__':
    sys.exit(main())
