#!/usr/bin/env python3
#
# Find the latest snapshot version for the specified branch.
#
# Example usage: ./latest_snapshot_version.py 7.x

import argparse
import requests


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('branch', type=str)
    args = parser.parse_args()

    r = requests.get('https://snapshots.elastic.co/latest/{}.json'.format(args.branch))
    r.raise_for_status()
    print(r.json()['version'])


if __name__ == '__main__':
    main()
