#!/usr/bin/env python3

from __future__ import print_function

import argparse

import requests
import os
import json
import jsondiff
import sys
try:
    from urlparse import urljoin, urlparse
except ImportError:
    from urllib.parse import urljoin, urlparse


def json_val(v1, v2):
    try:
        return json.loads(v1), json.loads(v2)
    except:
        return v1, v2


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


def iterate(val_id, key, v1, v2, apm_v1=True):
    ret_val = 0
    if isinstance(v1, dict) and isinstance(v2, dict):
        for k, v in v1.items():
            ret_val = max(ret_val, iterate(val_id, build_key(key, k), *json_val(v, v2[k] if k in v2 else ""),
                                           apm_v1=apm_v1))
        if ret_val == 0:
            for k, v in v2.items():
                ret_val = max(ret_val, iterate(val_id, build_key(key, k), *json_val(v, v1[k] if k in v1 else ""),
                                               apm_v1=not apm_v1))
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
                ret_val = max(ret_val, iterate(val_id, build_key(key, "{}={}".format(qkey, item[qkey])), item, item2,
                                               apm_v1=apm_v1))
        else:
            v1, v2 = sorted(v1), sorted(v2)
            for item1, item2 in zip(v1, v2):
                ret_val = max(ret_val, iterate(val_id, key, *json_val(item1, item2), apm_v1=apm_v1))
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
            v1_label, v2_label = "APM Server", "Kibana"
            if not apm_v1:
                v1_label, v2_label = v2_label, v1_label
            print("Value in {}: {!r}".format(v1_label, v1))
            print("Value in {}: {!r}".format(v2_label, v2))
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
    # TODO(axw) outsource to PyGitHub
    rsp = requests.get("https://api.github.com/repos/elastic/kibana/pulls")
    while rsp.status_code == 200:
        for pr in rsp.json():
            matches_branch = pr['base']['ref'] == branch
            matches_index_pattern_update = all(
                token in pr['title'].lower() for token in ['apm', 'update', 'index pattern'])
            if matches_branch and matches_index_pattern_update:
                return pr['head']['sha']
        # Parse "next" link
        links = requests.utils.parse_header_links(rsp.headers['link'])
        links = dict((link['rel'], link['url']) for link in links)
        next_url = links.get('next', None)
        if next_url is None:
            break
        rsp = requests.get(next_url)
    return None


def load_kibana_index_pattern_file(p):
    with open(p) as f:
        return json.load(f)


def load_kibana_index_pattern_url(p):
    rsp = requests.get(p)
    rsp.raise_for_status()
    return rsp.json()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--branch', default='master')
    parser.add_argument('-I', '--index-pattern',
                        default='src/plugins/apm_oss/server/tutorial/index_pattern.json',
                        help='index-pattern file path')
    parser.add_argument('-P', '--repo-path',
                        default='https://raw.githubusercontent.com/elastic/kibana/',
                        help='base repository path')
    parser.add_argument('-C', '--commit', help='Commit sha to get the index-pattern from')
    parser.add_argument("gen_index_pattern", type=argparse.FileType(mode="r"), help="expected index pattern")
    args = parser.parse_args()

    # load expected kibana index pattern from url or local file
    if args.repo_path.startswith("file://"):
        parsed = urlparse(args.repo_path)
        path = os.path.join(parsed.path, args.index_pattern)
        load_kibana_index_pattern = load_kibana_index_pattern_file
    else:
        ref = args.commit
        if ref is None:
            ref = get_kibana_commit(args.branch)
        if ref is None:
            ref = args.branch
        path = urljoin(args.repo_path, "/".join([ref, args.index_pattern]))
        load_kibana_index_pattern = load_kibana_index_pattern_url

    # load index pattern sync'd to kibana
    print("---- Comparing Generated Index Pattern with " + path)
    sync_index_pattern = load_kibana_index_pattern(path)

    # load generated index pattern
    gen_index_pattern = json.load(args.gen_index_pattern)["objects"][0]
    gen_index_pattern["attributes"].pop("title")  # detects title set in sync'd pattern

    # decode fields, they are json
    sync_index_pattern["attributes"]["fields"] = json.loads(sync_index_pattern["attributes"].pop("fields"))
    gen_index_pattern["attributes"]["fields"] = json.loads(gen_index_pattern["attributes"].pop("fields"))

    exit_val = 0
    exit_val = max(exit_val, iterate(sync_index_pattern["id"], "", gen_index_pattern, sync_index_pattern))

    # double check that there is no difference
    if exit_val == 0:
        d = jsondiff.JsonDiffer(syntax='symmetric').diff(sync_index_pattern, gen_index_pattern)
        if d:
            print("index patterns differ: ", d)
            return 5
        print("up-to-date")

    return exit_val


if __name__ == '__main__':
    sys.exit(main())
