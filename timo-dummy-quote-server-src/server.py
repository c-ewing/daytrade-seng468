import socketserver
import random
import time

class QuoteServerHandler(socketserver.BaseRequestHandler):
    """
    The request handler class for our server.

    It is instantiated once per connection to the server, and must
    override the handle() method to implement communication to the
    client.
    """

    def handle(self):
        # self.request is the TCP socket connected to the client
        self.data = self.request.recv(1024).strip()
        print("{} wrote: {}".format(self.client_address[0], self.data))

        call = self.data.decode("utf-8").split(" ")

        # Generate the response and send it back to the client
        random_price = random.randint(50, 500) / 100.0
        response = "{},{},{},{},{}".format(random_price, call[0], call[1], "1641952356940", "Tnssjq2UKzc+KQ/KhjmENlfJSHRD7VBXxiYh1CVpyDo=")
        # {price,symbol,username,timestamp,cryptokey}
        print(response)
        self.request.sendall(response.encode("utf-8"))

if __name__ == "__main__":
    HOST, PORT = "localhost", 4444
    print("Starting server on port {}:{}\n CTRL-C to quit\n".format(HOST, PORT))
    
    with socketserver.TCPServer((HOST, PORT), QuoteServerHandler) as server:
        # Activate the server; this will keep running until you
        # interrupt the program with Ctrl-C
        server.serve_forever()
