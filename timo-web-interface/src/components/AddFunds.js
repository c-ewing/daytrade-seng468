import Form from 'react-bootstrap/Form';
import InputGroup from 'react-bootstrap/InputGroup';
import Button from 'react-bootstrap/Button'
import { useState } from 'react';

function AddFunds(props) {
    const [inputValue, setInputValue] = useState(0);
    
    const handleChange = (event) => {
        const newValue = event.target.value;
        if (isNaN(newValue)) {
            alert("Please enter a valid number!");
        } else {
            setInputValue(event.target.value);
        }
    };

    // On click, add given input to Account Balance
    const handleClick = () => {
        var newBalance = parseFloat(props.accountBalance) + parseFloat(inputValue);
        props.setBalance(newBalance.toFixed(2));
        
        var operation = `[1] ADD,${props.username},${parseFloat(inputValue)}`;

        // fetch('http://localhost:8080/send-data', {

        // headers: {
        //     'Content-Type': 'application/json;charset=UTF-8'
        //     },
        //     method: 'POST',
        //     body: JSON.stringify({data: operation})
        // }).then(function(response) {
        //     console.log(response.body)
        // }).catch(error => {
        //     console.error(error);
        // });

    };

    return (
        <div className='addFundsForm' style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
            <InputGroup className="mb-3" style={{ width: '25%', padding:'10px' }}>
                <InputGroup.Text>$</InputGroup.Text>
                <Form.Control type="text" value={inputValue} aria-label="addFundsInput" onChange={handleChange}/>
            </InputGroup>
            <Button onClick={handleClick} variant="primary">Add Funds</Button>{' '}
        </div>

    );
}

export default AddFunds;