#!/usr/bin/env python

from __future__ import print_function

import argparse
import json
import logging
import sys

import requests


def beats_version(vendor):
    """parse vendor.json content and extract beats version"""
    v = json.load(vendor)
    found = set()
    for package in v['package']:
        if package['path'].startswith('github.com/elastic/beats/'):
            found.add(package['revision'])
    if len(found) == 0:
        raise Exception("no beats revision found")
    if len(found) > 1:
        raise Exception("multiple beats revisions found:\n" + "\n".join(sorted(found)))
    return found.pop()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-i', '--vendor', type=argparse.FileType('r'), default='vendor/vendor.json',
                        help='path to vendor.json')
    parser.add_argument('-B', '--beats-revision', type=str, help='beats revision override (skip reading vendor.json)')
    parser.add_argument('-D', '--debug', action='store_true')
    parser.add_argument("ref")
    args = parser.parse_args()

    level = logging.DEBUG if args.debug else logging.ERROR
    logging.basicConfig(level=level)

    current = args.beats_revision if args.beats_revision else beats_version(args.vendor)
    logging.debug("checking if latest beats revision in '%s' is: %s", args.ref, current)

    rsp = requests.get(
        "https://api.github.com/repos/elastic/beats/commits/{}".format(args.ref),
        headers={
            'Accept': 'application/vnd.github.VERSION.sha',
            'Etag': current,
        },
    )
    actual = rsp.content.decode('utf-8')
    if rsp.status_code != 200:
        print("failed to query latest beat revision on '{}': {}".format(args.ref, actual))
        return 1

    if actual != current:
        print("out of date: https://github.com/elastic/beats/compare/{}...{}".format(current, actual))
        return 1

    print("update to date on '{}' at {}".format(args.ref, actual))
    return 0


if __name__ == '__main__':
    sys.exit(main())
