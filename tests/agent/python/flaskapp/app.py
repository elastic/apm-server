# -*- coding: utf-8 -*-

import os

from flask import Flask, render_template, request, redirect, url_for
from elasticapm.contrib.flask import ElasticAPM

app = Flask(__name__)
app.debug = False

app.config['ELASTIC_APM'] = {
    'DEBUG': True,
}

apm = ElasticAPM(
    app,
    app_name='test-app',
    secret_token='',
)


@app.route('/')
def index():
    return 'OK'


@app.route('/error')
def error():
    raise ValueError(request.args.get('error', ''))


if __name__ == '__main__':
    port = 8081
    app.run(host='localhost', port=port)


