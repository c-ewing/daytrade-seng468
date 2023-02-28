import transactionData from './../test_transaction_data.json';

function JsonDataDisplay (props) {
    const transactionTable=props.data.map(
        (info)=>{
            return(
                <tr>
                    <td>{info.transaction_number}</td>
                    <td>{info.transaction_action.operation}</td>
                    <td>{info.transaction_action.SYM}</td>
                    <td>{info.transaction_action.numShares}</td>
                    <td>$ {info.transaction_action.quote_price}</td>
                    <td>$ {info.transaction_action.total_transaction_amount}</td>
                    <td>{info.transaction_start}</td>
                    <td>{info.transaction_completed}</td>
                </tr>
            )
        }
    )

    return(
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center'}}>
            <table className="table table-striped" style={{ width:'90%'}}>
                <thead>
                    <tr>
                    <th>Transaction ID</th>
                    <th>Operation</th>
                    <th>Ticker</th>
                    <th>Shares</th>
                    <th>Quote Price</th>
                    <th>Transaction Amount</th>
                    <th>Start Time</th>
                    <th>Completion Time</th>
                    </tr>
                </thead>
                <tbody>
                 
                    
                    {transactionTable}
                    
                </tbody>
            </table>
             
        </div>
    )
}

function TransactionHistory (props) {
    //TODO: get transaction  history from database
    const getTransactionHistory = () => {
        return transactionData;
    }
    return (
        <div className='row transactionTable'>
            <JsonDataDisplay data={getTransactionHistory()}/>
        </div>
    );
}

export default TransactionHistory;