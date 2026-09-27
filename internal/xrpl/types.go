package xrpl

// RPCRequest represents a standard XRPL JSON-RPC request envelope.
type RPCRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// AccountTxParams parameters for the "account_tx" method.
type AccountTxParams struct {
	Account        string      `json:"account"`
	LedgerIndexMin interface{} `json:"ledger_index_min,omitempty"`
	LedgerIndexMax interface{} `json:"ledger_index_max,omitempty"`
	Limit          int         `json:"limit,omitempty"`
	Forward        bool        `json:"forward,omitempty"`
}

// AccountTxResponse represents the top-level response from "account_tx".
type AccountTxResponse struct {
	Result AccountTxResult `json:"result"`
}

// AccountTxResult contains the transaction history for an account.
type AccountTxResult struct {
	Account            string           `json:"account"`
	Status             string           `json:"status"`
	ErrorMessage       string           `json:"error_message,omitempty"`
	Error              string           `json:"error,omitempty"`
	Transactions       []AccountTxEntry `json:"transactions"`
	Validated          bool             `json:"validated"`
	LedgerIndexMin     int64            `json:"ledger_index_min,omitempty"`
	LedgerIndexMax     int64            `json:"ledger_index_max,omitempty"`
	LedgerCurrentIndex int64            `json:"ledger_current_index,omitempty"`
}

// AccountTxEntry wraps a single transaction and its execution metadata.
type AccountTxEntry struct {
	Validated   bool        `json:"validated"`
	LedgerIndex uint32      `json:"ledger_index,omitempty"`
	Tx          Transaction `json:"tx"`
	Meta        TxMeta      `json:"meta"`
}

// Transaction represents the core fields of an XRPL transaction.
type Transaction struct {
	Account         string `json:"Account"`
	Destination     string `json:"Destination"`
	DestinationTag  uint32 `json:"DestinationTag"`
	TransactionType string `json:"TransactionType"`
	Hash            string `json:"hash"`
	Date            int64  `json:"date"`
	LedgerIndex     uint32 `json:"ledger_index"`
	InLedger        uint32 `json:"inLedger,omitempty"`
}

// TxMeta represents the transaction execution result in the ledger.
type TxMeta struct {
	TransactionResult string      `json:"TransactionResult"`
	DeliveredAmount   interface{} `json:"delivered_amount"`
}
