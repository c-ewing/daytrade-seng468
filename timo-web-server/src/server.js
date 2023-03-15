const express = require("express");
const cors = require('cors');
const app = express();
const bodyParser = require('body-parser');
const amqp = require('amqplib/callback_api')

// Send data to Rabbit Queue
const sendToRabbit = (data) => {
  console.log(data)
}
const corsOptions = {
  origin: 'http://localhost',
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
  const operations = data.split("\r\n")
  for (var i = 0; i < operations.length; i++) {
    sendToRabbit(operations[i])
  }
  
}
);

app.listen(8080, () => {
    console.log("Server running on 8080")
}
);