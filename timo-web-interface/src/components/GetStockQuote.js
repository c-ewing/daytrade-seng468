import Form from 'react-bootstrap/Form';
import InputGroup from 'react-bootstrap/InputGroup';
import Button from 'react-bootstrap/Button'
import { useState } from 'react';
import BuyStock from './BuyStock';

function GetStockQuote(props) {
    const [stockSym, setStockSym] = useState('');
    const [stockPrice, setStockPrice] = useState(null);
    const [canBuy, setCanBuy] = useState(false);

    const handleChange = (event) => {
        setStockSym(event.target.value);
    };

    const handleClick = () => {
        // Get Stock quote from server
        // TODO
        // If Stock exists, show price and render buy option. If not, give alert
        var operation = `[1] QUOTE,${props.username},${stockSym}`; 
        setStockPrice(1.12);
        setCanBuy(true);
    };

    return(
        <div className='getStockPrices'>
            <div className='getStockForm' style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
                <InputGroup style={{ width: '25%', padding:'10px'}}>
                    <InputGroup.Text>Stock Ticker</InputGroup.Text>
                    <Form.Control type="text" aria-label="stockSymbol" onChange={handleChange}/>
                </InputGroup>
                <Button onClick={handleClick} variant="primary">Get Price</Button>{' '}

            </div>

            <div className='buyStock' style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
                {canBuy && <BuyStock stockSym={stockSym} stockPrice={stockPrice} username={props.username}
                                        accountBalance={props.accountBalance} setBalance={props.setBalance}/>}
            </div>
        </div>
    );
}

export default GetStockQuote;