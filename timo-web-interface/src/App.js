import logo from './logo.png';
import './App.css';
import DropdownButton from 'react-bootstrap/DropdownButton';
import Dropdown from 'react-bootstrap/Dropdown';
import { useState } from 'react';
import AddFunds from './components/AddFunds'
import GetStockQuote from './components/GetStockQuote';
import ManageStocks from './components/ManageStocks';
import TransactionHistory from './components/TransactionHistory';

function App() { 
  const [accountBalance, setBalance] = useState(0);
  const username = "Ted"

  const [selectedAction, setSelectedActions] = useState(null);
  const handleItemClick = (action) => {
    setSelectedActions(action);
  };

  const getAccountDetails = (username) => {
    //TODO: Query database for the Account details (ie. Balance and Stocks)
  }

  return (
    
    <div className="App">

      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
      </header>

      <body>
        <h2>Account Balance</h2>
        <h3>$ {accountBalance}</h3>
          <DropdownButton variant="success" id="dropdown-basic-button" title="Actions">
            <Dropdown.Item onClick={() => handleItemClick('AddFunds')}>Add Funds</Dropdown.Item>
            <Dropdown.Item onClick={() => handleItemClick('GetStockQuote')}>Get Stock Quote</Dropdown.Item>
            <Dropdown.Item onClick={() => handleItemClick('ManageStocks')}>Manage Stocks</Dropdown.Item>
            <Dropdown.Item onClick={() => handleItemClick('TransactionHistory')}>Transaction History</Dropdown.Item>
          </DropdownButton>

          {selectedAction === 'AddFunds' && <AddFunds username={username} accountBalance={accountBalance} setBalance={setBalance}/>}
          {selectedAction === 'GetStockQuote' && <GetStockQuote username={username} accountBalance={accountBalance} setBalance={setBalance}/>}
          {selectedAction === 'ManageStocks' && <ManageStocks username={username} accountBalance={accountBalance} setBalance={setBalance}/>}
          {selectedAction === 'TransactionHistory' && <TransactionHistory username={username}/>}
      </body>
    </div>
    
  );
}

export default App;