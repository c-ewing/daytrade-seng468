import Form from "react-bootstrap/Form"
import Button from "react-bootstrap/Button"
import { useState } from "react"

const ampq = require('amqplib/callback_api');

const sendOperationFile = (msg) => {
  //TODO: Add connection to Rabbit
  ampq.connect('connection here', (err, connection) => {
    if (err) {
      throw err;
    }
    connection.createChannel((err, channel) => {
      if (err) {
        throw err;
      }

      // TODO: Change queue name
      let queueName = "operation_queue";

      channel.assertQueue(queueName,  {
        durable: false
      });

      channel.sendToQueue(queueName, Buffer.from(msg));
    })
  }
  )
}

function FileUpload(props) {

  const [selectedFile, setSelectedFile] = useState(null);

  const handleSubmit = (event) => {
    event.preventDefault();
    if (selectedFile) {
      const reader = new FileReader();
      reader.readAsText(selectedFile);
      reader.onload = () => {
        console.log(reader.result);
        sendOperationFile(reader.result)
      };
    }
  };

  return (
    <div>
      <Form.Group controlId="formFile" className="mb-3" id="file-selector" >
        <Form.Label>Test File Input</Form.Label>
        <Form.Control type="file" onChange={(event) => setSelectedFile(event.target.files[0])}/>
      </Form.Group>
      <Button variant="success" type="submit" onClick={handleSubmit}>Run</Button>
    </div>
  )
}

export default FileUpload
