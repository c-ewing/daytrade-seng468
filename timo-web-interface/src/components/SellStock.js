import Form from 'react-bootstrap/Form';
import InputGroup from 'react-bootstrap/InputGroup';
import Button from 'react-bootstrap/Button'
import { useState } from 'react'; 


function SellStock (props) {
    const [numShares, setNumShares] = useState(0);
    const [isLimitOrder, setLimitOrder] = useState(false);
    const [priceTrigger, setPriceTrigger] = useState(0);

    // Set the number of shares to sell
    const updateNumShares = (event) => {
        if (event.target.value === props.shares) return;
        setNumShares(event.target.value);
    };

    const addShare = () => {
        if (numShares === props.shares) return;
        setNumShares(parseInt(numShares) + 1);
    };

    const subtractShare = () => {
        if (numShares === 0) return;
        setNumShares(parseInt(numShares) - 1);
    }

    // Sell the shares 
    // Update database TODO
    const confirmSell = () => {
        // If limit sell, only sell on price trigger TODO
        if (isLimitOrder) {
            if (isLimitOrder <= 0 || isNaN(priceTrigger)) {
                alert(`Invalid price trigger`);
            }
            alert(`Limit Sell for ${numShares} of ${props.stockSym} at $ ${priceTrigger}`)
            return;
        }
        
        // Update account balance
        // Update database TODO
        const fullPrice = numShares * parseFloat(props.stockPrice);
        alert(`Sold ${numShares} shares of ${props.stockSym} at $ ${props.stockPrice}. ${fullPrice}`);
        const newBalance = parseFloat(props.accountBalance) + parseFloat(fullPrice);
        props.setBalance(newBalance.toFixed(2));
    }

    const handleCheckbox = (event) => {
        setLimitOrder(event.target.checked);
    }

    const setTrigger = (event) => {
        setPriceTrigger(event.target.value);
    }

    return(
        <div className='sellStockForm'>
            <div className='row'>
                <div className='col md-3'>
                    <h1>{props.stockSym.toUpperCase()} Price<br/></h1>
                    <h2>$ {props.stockPrice}</h2>
                </div>

                <div className='col md-9'>
                    <div className='row'>
                        <InputGroup className="mb-3">
                            <InputGroup.Text>Shares</InputGroup.Text>
                            <Form.Control aria-label="numShares" value={numShares} onChange={updateNumShares}/>
                            <Button variant="outline-secondary" onClick={addShare}>+</Button>
                            <Button variant="outline-secondary" onClick={subtractShare}>-</Button>
                        </InputGroup>
                    </div>
                    <div className='row'>
                        <InputGroup className="mb-3">
                            <InputGroup.Checkbox label='Sell Limit Order'aria-label="limitOrderOption" checked={isLimitOrder} onChange={handleCheckbox}/>
                            <InputGroup.Text>Sell Limit Order $</InputGroup.Text>
                            <Form.Control value={priceTrigger} aria-label="limitOrderValue" disabled={!isLimitOrder} onChange={setTrigger}/>
                        </InputGroup>
                    </div>
                </div>
            </div>
            <div className='row'>
                <Button onClick={confirmSell} variant="primary" disabled={numShares ===0}>Confirm Sell</Button>{' '}
            </div>
            
        </div>
    );

}

export default SellStock;