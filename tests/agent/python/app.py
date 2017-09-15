# -*- coding: utf-8 -*-

import os
import sys
import logging
import httplib

from flask import Flask, render_template, request, redirect, url_for
from elasticapm.contrib.flask import ElasticAPM


logging.basicConfig(stream=sys.stdout, level=logging.INFO)
httplib.HTTPConnection.debuglevel = 2
logging.getLogger('httplib').setLevel(logging.INFO)

app = Flask(__name__)

app.config['ELASTIC_APM'] = {
    'APP_NAME': 'flask-app',
    'SECRET_TOKEN': '',
}

apm = ElasticAPM(app)


@app.route('/')
def index():
    return 'OK'


@app.route('/error')
def error():
    raise ValueError(request.args.get('error', ''))


if __name__ == '__main__':
    app.run(host='0.0.0.0')
