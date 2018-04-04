#!/usr/bin/env python
"""
Generate data used to put the correct revision information in NOTICE.TXT.
"""
import argparse
import json
import logging
import sys


def gather(vendor):
    overrides = {}
    for package in vendor['package']:
        path = package['path']
        sub_path = []
        for p in path.split('/'):
            sub_path.append(p)
            sub_path_str = '/'.join(sub_path)
            if sub_path_str in overrides and overrides[sub_path_str]['revision'] != package['revision'] and len(sub_path) > 1:
                logging.debug("warning: %s changed by: %s was: %s is: %s", sub_path_str,
                              path, overrides[sub_path_str]['revision'], package['revision'])
            overrides[sub_path_str] = package
    return overrides


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-i', '--vendor_input', type=argparse.FileType('r'), default='_beats/vendor/vendor.json')
    parser.add_argument('-o', '--overrides', type=argparse.FileType('w'), default=sys.stdout)
    parser.add_argument('-D', '--debug', action='store_true')
    args = parser.parse_args()

    level = logging.DEBUG if args.debug else logging.ERROR
    logging.basicConfig(level=level)

    vendor = json.load(args.vendor_input)
    overrides = gather(vendor)
    for path_override, package in sorted(overrides.items()):
        vendor['package'].append({
            "path": path_override,
            "revision": package["revision"]
        })
    json.dump(vendor, args.overrides)


if __name__ == '__main__':
    main()
