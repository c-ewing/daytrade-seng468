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
  origin: 'http://localhost:3000',
}

// Configure Express
const configuredCors = cors(corsOptions);
app.options('*', configuredCors)
app.use(cors())
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Operation File received
app.post('/send-data', configuredCors, (req, res) => {
  const data = req.body.data;
  const operations = data.split('\r\n')
  for (var i = 0; i < operations.length; i++) {
    sendToRabbit(operations[i])
  }
}
);

app.listen(3001, () => {
    console.log("Server running on 3001")
}
);