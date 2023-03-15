import pika
import json
import uuid


def test_well_formed_quote_command():
    # Craft the command to send to the quote server
    command = json.dumps({"command": "QUOTE", "userid": "umhaEY4lil", "stock_symbol": "BKM"})
    
    # Send a call to the quote drive to get a quote from the quote server
    response = QuoteRPC().call(command)

    assert response["command"] == "QUOTE"
    assert response["userid"] == "umhaEY4lil"
    assert response["stock_symbol"] == "BKM"
    assert float(response["quote_price"]) >= 0.5 and float(response["quote_price"]) <= 5.0
    assert response["timestamp"] != ""
    assert response["cryptographic_key"] != ""


class QuoteRPC(object):

    def __init__(self):
        self.connection = pika.BlockingConnection(
            pika.ConnectionParameters(host='localhost'))

        self.channel = self.connection.channel()

        result = self.channel.queue_declare(queue='', exclusive=True)
        self.callback_queue = result.method.queue

        self.channel.basic_consume(
            queue=self.callback_queue,
            on_message_callback=self.on_response,
            auto_ack=True)

        self.response = None
        self.corr_id = None

    def on_response(self, ch, method, props, body):
        if self.corr_id == props.correlation_id:
            self.response = body

    def call(self, n):
        self.response = None
        self.corr_id = str(uuid.uuid4())
        self.channel.basic_publish(
            exchange='',
            routing_key='quote_price_requests',
            properties=pika.BasicProperties(
                reply_to=self.callback_queue,
                correlation_id=self.corr_id,
            ),
            body=str(n))
        self.connection.process_data_events(time_limit=None)
        return json.loads(self.response)



