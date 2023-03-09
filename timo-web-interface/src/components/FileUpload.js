import Form from "react-bootstrap/Form"
import Button from "react-bootstrap/Button"
import { useState } from "react"

function FileUpload(props) {

  const [selectedFile, setSelectedFile] = useState(null);

  const handleSubmit = (event) => {
    console.log("Hello")
    event.preventDefault();
    if (selectedFile) {
      const reader = new FileReader();
      reader.readAsText(selectedFile);
      reader.onload = () => {
        console.log(reader.result);
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