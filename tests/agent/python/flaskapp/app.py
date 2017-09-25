# -*- coding: utf-8 -*-

import os
import elasticapm
from flask import Flask
from elasticapm.contrib.flask import ElasticAPM

app = Flask(__name__)
app.debug = False

app.config['ELASTIC_APM'] = {
    'DEBUG': True,
    'TRACES_SEND_FREQ': 3
}

apm = ElasticAPM(
    app,
    app_name='test-app',
    secret_token='',
)


@app.route('/')
def index():
    return 'OK'


@app.route('/foo')
def foo_route():
    return foo()


@elasticapm.trace()
def foo():
    return "OK"


@app.route('/bar')
def bar_route():
    return bar()


@elasticapm.trace()
def bar():
    return "OK"


if __name__ == '__main__':
    port = 8081
    app.run(host='localhost', port=port)


