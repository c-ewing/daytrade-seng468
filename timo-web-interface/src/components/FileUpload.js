import Form from "react-bootstrap/Form"
import Button from "react-bootstrap/Button"
import { useState } from "react"
import rabbit from 'rabbit.js';

const sendOperationFile = (msg) => {
  //TODO: Add connection to Rabbit
  const context = rabbit.createContext('ws://localhost:15674/ws');

  // Create a socket
  const socket = context.socket('PUSH');

  // Connect to a queue
  socket.connect('my_queue', () => {
    // Send a message
    socket.write('Hello, world!');
  });
  
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
