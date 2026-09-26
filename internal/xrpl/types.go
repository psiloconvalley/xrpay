package xrpl

// RPCRequest represents a standard XRPL JSON-RPC request envelope.
type RPCRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// AccountTxParams parameters for the "account_tx" method.
type AccountTxParams struct {
	Account        string `json:"account"`
	LedgerIndexMin int64  `json:"ledger_index_min"` // -1 for earliest available
	LedgerIndexMax int64  `json:"ledger_index_max"` // -1 for latest available
	Limit          int    `json:"limit,omitempty"`
	Forward        bool   `json:"forward,omitempty"` // true = chronological order
}

// AccountTxResponse represents the top-level response from "account_tx".
type AccountTxResponse struct {
	Result AccountTxResult `json:"result"`
}

// AccountTxResult contains the transaction history for an account.
type AccountTxResult struct {
	Account      string           `json:"account"`
	Status       string           `json:"status"`
	ErrorMessage string           `json:"error_message,omitempty"`
	Error        string           `json:"error,omitempty"`
	Transactions []AccountTxEntry `json:"transactions"`
	Validated    bool             `json:"validated"`
}

// AccountTxEntry wraps a single transaction and its execution metadata.
type AccountTxEntry struct {
	Validated bool        `json:"validated"`
	Tx        Transaction `json:"tx"`
	Meta      TxMeta      `json:"meta"`
}

// Transaction represents the core fields of an XRPL transaction.
type Transaction struct {
	Account         string `json:"Account"`          // Sender
	Destination     string `json:"Destination"`      // Receiver (Merchant)
	DestinationTag  uint32 `json:"DestinationTag"`   // Routing Tag
	TransactionType string `json:"TransactionType"`  // "Payment"
	Hash            string `json:"hash"`             // 64-character hex tx hash
	Date            int64  `json:"date"`             // Ripple epoch timestamp
	LedgerIndex     uint32 `json:"ledger_index"`
}

// TxMeta represents the transaction execution result in the ledger.
type TxMeta struct {
	TransactionResult string      `json:"TransactionResult"` // "tesSUCCESS"
	DeliveredAmount   interface{} `json:"delivered_amount"`  // string for XRP drops, object for tokens
}
