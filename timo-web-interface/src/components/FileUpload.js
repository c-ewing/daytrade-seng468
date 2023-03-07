import Form from "react-bootstrap/Form"
import InputGroup from "react-bootstrap/InputGroup"
import Button from "react-bootstrap/Button"
import {useState} from "react"

function FileUpload(props) {
  return (
    <>
      <Form.Group controlId="formFile" className="mb-3">
        <Form.Label>File Input</Form.Label>
        <Form.Control type="file" />
      </Form.Group>
    </>
  )
}

export default FileUpload
