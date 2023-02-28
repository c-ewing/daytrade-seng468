import Form from 'react-bootstrap/Form';
import InputGroup from 'react-bootstrap/InputGroup';
import Button from 'react-bootstrap/Button'
import { useState } from 'react';
import stockData from './../test_stock_data.json';
import SellStock from './SellStock';

function JsonDataDisplay (props) {
    const stockTable=props.data.map(
        (info)=>{
            return(
                <tr>
                    <td>{info.SYM}</td>
                    <td>{info.numShares}</td>
                    <td>$ {info.Price}</td>
                </tr>
            )
        }
    )

    return(
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
            <table className="table table-striped" style={{ width:'75%'}}>
                <thead>
                    <tr>
                    <th>Ticker</th>
                    <th>Shares Owned</th>
                    <th>Price</th>
                    </tr>
                </thead>
                <tbody>
                 
                    
                    {stockTable}
                    
                </tbody>
            </table>
             
        </div>
    )
}

// Show the currently owned stocks of the user
function ManageStocks (props) {
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
        setStockPrice(1.12);
        setCanBuy(true);
    };

    // TODO: Get Stock data from database 
    const getUserStocks = () => {
        return stockData;
    }

    return (
        <div>
            <div className='row stockTable'>
                <JsonDataDisplay data={getUserStocks()}/>
            </div>
            <div className='row sellStocks'>
                <div className='getStockForm' style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
                    <InputGroup style={{ width: '25%', padding:'10px'}}>
                        <InputGroup.Text>Stock Ticker</InputGroup.Text>
                        <Form.Control type="text" aria-label="stockSymbol" onChange={handleChange}/>
                    </InputGroup>
                    <Button onClick={handleClick} variant="primary">Get Price</Button>{' '}

                </div>
                <div className='sellStock' style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
                    {canBuy && <SellStock stockSym={stockSym} stockPrice={stockPrice} username={props.username}
                                        accountBalance={props.accountBalance} setBalance={props.setBalance} shares={6}/>}
                </div>
            </div>
            
        </div>
    )
}

export default ManageStocks;