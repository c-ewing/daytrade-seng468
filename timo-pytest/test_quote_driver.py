import pika
import json


def test_well_formed_quote_command():
    connection = pika.BlockingConnection(
        pika.ConnectionParameters(host='localhost'))
    channel = connection.channel()

    # Declare the queue for the request
    channel.queue_declare(queue='quote_price_requests', durable=True)
    # Create a temporary queue for the response
    return_queue = channel.queue_declare(queue='', exclusive=True)
    # Craft the command to send to the quote server
    command = json.dumps({"Command": "QUOTE", "Userid": "umhaEY4lil", "StockSymbol": "BKM"}).encode("utf-8")
    
    # Send a call to the quote drive to get a quote from the quote server
    channel.basic_publish(exchange='', routing_key='quote_price_requests', properties=pika.BasicProperties(reply_to=return_queue.method.queue), body=command)

    response = ''
    # Wait for the response
    for method_frame, properties, body in channel.consume(return_queue.method.queue, inactivity_timeout=10):
        # Display the message
        response = body.decode("utf-8")
        # Acknowledge the message
        channel.basic_ack(method_frame.delivery_tag)
        # Escape out of the loop after one iteration
        break

    response = json.loads(response)

    assert response["Command"] == "QUOTE"
    assert response["Userid"] == "umhaEY4lil"
    assert response["StockSymbol"] == "BKM"
    assert float(response["QuotePrice"]) >= 0.5 and float(response["QuotePrice"]) <= 5.0
    assert response["Timestamp"] != ""
    assert response["CryptographicKey"] != ""



