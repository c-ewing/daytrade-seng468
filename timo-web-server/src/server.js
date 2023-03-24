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
    console.log("Successfully conected to rabbit on: ", rabbitConnect);

    conn.createChannel((err, channel) => {
      if (err) throw err;
      
      channel.assertQueue(commandQueue);
      console.log("Successfully connected to: ", commandQueue);
      channel.prefetch(10);
      for (var i = 0; i < data.length; i++) {
        channel.sendToQueue(commandQueue, Buffer.from(JSON.stringify(data[i])), {mandatory:true}, (sendErr, ok) => {
          if (sendErr) throw sendErr;
          console.log("Sent operation");
          // channel.waitForConfirms((confirmErr) => {
          //   if (confirmErr) throw confirmErr;
          //   console.log('All messages confirmed: ', data[i]);
          // });
        });
      }
    });
  });
}

const corsOptions = {
  origin: 'http://localhost'
}

// Configure Express
const configuredCors = cors(corsOptions);
app.options('*', configuredCors)
app.use(cors())
app.use(bodyParser.json({limit: '10mb'}));
app.use(bodyParser.urlencoded({ extended: true, limit: '10mb' }));

// Operation File received
app.post('/send-data', configuredCors, (req, res) => {
  const data = req.body.data;
  const operations = data.split("\r\n");
  // Remove last empty line 
  operations.pop();
  for (var i = 0; i < operations.length; i++) operations[i] = formatOperation(operations[i]);
  sendToRabbit(operations);
}
);

app.get('/', configuredCors, (req, res) => {
  res.send("Server Healthty")
});

app.listen(8080, () => {
    console.log("Server running on 8080")
}
);