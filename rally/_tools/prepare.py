#!/usr/bin/env python

import argparse
import os
import bz2
import json
from shutil import rmtree
from datetime import datetime, timedelta

try:
    from urllib.request import urlretrieve
except ImportError:
    from urllib import urlretrieve

DATE_PATTERN = '%Y-%m-%d'


class File(object):

    def __init__(self, name, base_url, base_dir):
        self.name = name
        self.compr = "bz2"
        self.fmt = "json"
        self.url = "{}/{}.{}.{}".format(base_url, name, self.fmt, self.compr)
        self.path = "{}.{}".format(os.path.join(base_dir, name), self.fmt)
        self.path_compr = "{}.{}".format(self.path, self.compr)


class Downloader(object):

    def __init__(self, args, path):
        self.tmp = path
        self.files = [File("{}_base".format(ev), args.url, self.tmp) for ev in args.events]

    def download(self, f):
        print("Downloading compressed data for {}".format(f.name))
        try:
            urlretrieve(f.url, f.path_compr)
        except Exception as e:
            print("Error downloading corpus for {}: {}".format(f.name, e))

    def decompress(self, f):
        print("Decompressing data for {}".format(f.name))
        cf = bz2.BZ2File(f.path_compr, "r")
        try:
            with open(f.path, 'wb') as f:
                for data in iter(lambda: cf.read(100 * 1024), b''):
                    f.write(data)
        except Exception as e:
            print("Error decompressing corpus for {}: {}".format(f.name, e))
        finally:
            cf.close()

    def run(self):
        create_dir(self.tmp)
        for f in self.files:
            self.download(f)
            self.decompress(f)


class Corpora(object):
    def __init__(self, args, input_path, target_path):
        self.events = args.events
        self.days = args.days
        self.inp_path = input_path
        self.target_path = target_path
        self.time_fmt = '%Y-%m-%dT%H:%M:%S.%fZ'
        self.start_day = datetime.strptime(args.start_date, DATE_PATTERN)

    def exists(self, inp, keys):
        for k in keys:
            if k not in inp:
                return False
            inp = inp[k]
        return True

    def update_id(self, doc, name, val):
        if self.exists(doc, [name, 'id']):
            doc[name]['id'] = "{}{}".format(doc[name]['id'], val)

    def updated_date(self, start, days_back):
        d = start + timedelta((self.start_day - timedelta(days_back) - start).days)
        return d.strftime(self.time_fmt)

    def process(self):
        create_dir(self.target_path)

        for ev in self.events:
            inp = "{}_base.json".format(os.path.join(self.inp_path, ev))
            if not os.path.exists(inp):
                print("Warning: input file {} does not exist".format(inp))
                continue
            for idx in range(self.days):
                out = "{}-{}.json".format(os.path.join(self.target_path, ev), idx)
                print("Processing {}".format(out))
                with open(out, 'w') as w:
                    with open(inp) as r:
                        for line in r:
                            doc = json.loads(line)
                            self.update_id(doc, 'transaction', idx)
                            self.update_id(doc, 'error', idx)
                            self.update_id(doc, 'span', idx)
                            d = datetime.strptime(doc['@timestamp'], self.time_fmt)
                            doc['@timestamp'] = self.updated_date(d, idx)
                            w.write(json.dumps(doc))
                            w.write("\n")


def create_dir(path, rm=False):
    if rm and os.path.exists(path):
        rmtree(path)
    if not os.path.exists(path):
        os.makedirs(path)


class Args(object):

    def __init__(self):
        parser = argparse.ArgumentParser()
        parser.add_argument('--url',
                            default='http://benchmarks.elasticsearch.org.s3.amazonaws.com/corpora/apm',
                            help='base URL for downloading corpora')
        parser.add_argument('--events',
                            default=['error', 'transaction', 'span'],
                            nargs='+',
                            help='events for which corpus will be prepared')
        parser.add_argument('--days',
                            type=int,
                            default=10,
                            help='number of days the corpora should be created for')
        parser.add_argument('--start-date',
                            dest='start_date',
                            default=datetime.today().strftime(DATE_PATTERN),
                            help='must be in format YY-mm-dd, data will be created backwards from the start_date')
        parser.add_argument('--skip-download',
                            dest='skip_download',
                            action='store_true',
                            help='skip download and extracting of corpora')
        parser.add_argument('--es-data',
                            dest='es_data',
                            help='path to data directory, only used when skip-download is set')
        parser.add_argument('--corpora',
                            help='path to corpora directory')
        self.p = parser

    def setup(self):
        return self.p.parse_args()


if __name__ == '__main__':
    try:
        args = Args().setup()
        cwd = os.path.dirname(os.path.realpath(__file__))
        es_path = args.es_data
        if not args.skip_download or es_path is None:
            es_path = os.path.join(cwd, "tmp")

        if not args.skip_download:
            Downloader(args, es_path).run()

        corpora_path = args.corpora if args.corpora else os.path.join(cwd, "..", "corpora")
        Corpora(args, es_path, corpora_path).process()
    except Exception as e:
        print(str(e))
