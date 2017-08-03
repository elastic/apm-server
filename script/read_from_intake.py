import datetime
import json
import socket
from functools import partial
from kombu import messaging, Connection, entity

import click


class JSONDateTimeEncoder(json.JSONEncoder):
    def default(self, o):
        if isinstance(o, datetime.datetime):
            return o.isoformat()
        return json.JSONEncoder.default(self, o)


def handle_message(body, message, app_uuid=None, output_file=None):
    message.ack()
    if app_uuid and app_uuid != body['kwargs']['data']['app_uuid']:
        return
    output_file.write(
        json.dumps(body['kwargs']['data'], cls=JSONDateTimeEncoder) + '\n')


@click.command()
@click.option('--output-file', type=click.File('wb'), help='File name to write to, - for stdout', default='-')
@click.option('--rabbitmq-url', envvar='READER_RABBITMQ_URL', help='URL of rabbitmq instance and vhost')
@click.option('--queue-name', envvar='READER_QUEUE_NAME', help='Name of queue')
@click.option('--exchange-name', help='Name of exchange', default='transactions')
@click.option('--max-messages', help='Maximum number of messages to fetch', default=None, type=int)
@click.option('--app-uuid', help='Filter specific app uuid', default=None)
def reader(output_file, rabbitmq_url, queue_name, exchange_name, max_messages, app_uuid):
    """
    Reads messages from RabbitMQ queue and writes them into a file or STDOUT
    """
    connection = Connection(rabbitmq_url)
    exchange = entity.Exchange(exchange_name)
    queue = entity.Queue(queue_name, exchange, queue_name, durable=False)
    consumer = messaging.Consumer(connection, queue, accept=['pickle'], callbacks=[
        partial(handle_message, app_uuid=app_uuid, output_file=output_file),
    ])
    consumer.consume()
    while True:
        try:
            connection.drain_events(timeout=5)
        except socket.timeout:
            pass


if __name__ == '__main__':
    reader()
