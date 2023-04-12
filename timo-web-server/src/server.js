const express = require("express");
const cors = require('cors');
const app = express();
const bodyParser = require('body-parser');
const amqp = require('amqplib/callback_api');
const commandQueue = 'command_queue';
const rabbitConnect = 'amqp://guest:guest@rabbitmq-dev:5672';

// Format operations for transaction server
const formatOperation = (data) => {
  var transactionNum = data.split(' ')[0];
  transactionNum = parseInt(transactionNum.substring(1, transactionNum.length - 1));
  var transaction = data.split(' ')[1].split(',');
  var command = transaction[0];
  var userID = transaction[1];
  var filename = null;
  var stockSym = null;
  var amount = null;

  if (['ADD'].includes(command)) {
    amount = parseFloat(transaction[2]);
  }

  if (['QUOTE', 'CANCEL_SET_BUY', 'CANCEL_SET_SELL'].includes(command)) {
    stockSym = transaction[2];
  }

  if (['BUY', 'SELL', 'SET_BUY_AMOUNT', 'SET_BUY_TRIGGER', 'SET_SELL_AMOUNT','SET_SELL_TRIGGER'].includes(command)) {
    stockSym = transaction[2];
    amount = parseFloat(transaction[3]);
  }

  if (command == 'DUMPLOG') {
    if (transaction.length == 2){
      filename = transaction[1];
      userID = null;
    } else {
      filename = transaction[2]
    }
  }

  const operationDict = {
    "command":              command,
    "transaction_number":   transactionNum,
    "userid":               userID,
    "stock_symbol":         stockSym,
    "amount":               amount, 
    "filename":             filename,
  }
  return operationDict;

}

const sendToRabbit = (data) => {
  amqp.connect(rabbitConnect, (err, conn) => {
    if (err) throw err;
    console.log("Successfully connected to RabbitMQ on: ", rabbitConnect);

    conn.createChannel((err, channel) => {
      if (err) throw err;

      // Create a new queue for acknowledgments
      const acknowledgmentQueue = 'acknowledgment_queue';
      channel.assertQueue(acknowledgmentQueue);

      // Bind the acknowledgment queue to the exchange
      const exchange = 'ack_exchange';
      channel.assertExchange(exchange, 'direct', { durable: true });

      const routingKey = 'your_routing_key';
      channel.bindQueue(acknowledgmentQueue, exchange, routingKey);

      channel.assertQueue(commandQueue);
      console.log("Successfully connected to: ", commandQueue);
      channel.prefetch(10);

      for (var i = 0; i < data.length; i++) {
        channel.sendToQueue(commandQueue, Buffer.from(JSON.stringify(data[i])), {mandatory:true}, (sendErr, ok) => {
          if (sendErr) throw sendErr;
          console.log("Sent operation");

          // Set up a consumer to receive acknowledgments
          channel.consume(acknowledgmentQueue, (msg) => {
            console.log(`Received acknowledgment: ${msg.content.toString()}`);
            // Do something with the acknowledgment message
          }, { noAck: false });
        });
      }
    });
  });
}



const corsOptions = {
  origin: true
}

// Configure Express
const configuredCors = cors(corsOptions);
app.options('*', configuredCors)
app.use(cors())
app.use(bodyParser.json({limit: '10mb'}));
app.use(bodyParser.urlencoded({ extended: true, limit: '10mb' }));

// Operation File received
app.post('/send-data', configuredCors, (req, res) => {
  var startTime = new Date();
  const data = req.body.data;
  const operations = data.split("\r\n");
  // Remove last empty line 
  operations.pop();
  for (var i = 0; i < operations.length; i++) operations[i] = formatOperation(operations[i]);
  
  var numAcks = operations.length;
  sendToRabbit(operations);
  var endTime = new Date();
  var timeDif = endTime - startTime;
  var TPS = (numAcks/timeDif) * 1000;
  console.log(`Operations took ${timeDif} ms: TPS = ${TPS}`)
}
);

app.get('/', configuredCors, (req, res) => {
  res.send("Server Healthty")
});

app.listen(8080, () => {
    console.log("Server running on 8080")
}
);