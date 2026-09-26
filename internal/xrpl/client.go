package xrpl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

// Public XRPL Endpoints
const (
	TestnetRPCURL = "https://s.altnet.rippletest.net:51234"
	MainnetRPCURL = "https://xrplcluster.com"
)

var (
	ErrRPCRequestFailed  = errors.New("xrpl rpc request failed")
	ErrAccountNotFound   = errors.New("account not found on xrpl")
	ErrTransactionFailed = errors.New("transaction did not succeed on ledger")
)

// PaymentEvent represents a verified incoming payment extracted from the ledger.
type PaymentEvent struct {
	TxHash         string
	Sender         string
	Destination    string
	DestinationTag uint32
	DeliveredDrops domain.Drops
	LedgerIndex    uint32
	Validated      bool
}

// Client interacts with the XRPL JSON-RPC API.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// NewClient creates an XRPL JSON-RPC client with a strict timeout.
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetAccountPayments fetches validated incoming XRP payments for an address.
func (c *Client) GetAccountPayments(ctx context.Context, account string, minLedger int64) ([]PaymentEvent, error) {
	reqBody := RPCRequest{
		Method: "account_tx",
		Params: []interface{}{
			AccountTxParams{
				Account:        account,
				LedgerIndexMin: minLedger,
				LedgerIndexMax: -1,
				Forward:        true,
			},
		},
	}

	jsonPayload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rpc request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRPCRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: http status %d", ErrRPCRequestFailed, resp.StatusCode)
	}

	var rpcResp AccountTxResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode xrpl response: %w", err)
	}

	if rpcResp.Result.Error != "" {
		if rpcResp.Result.Error == "actNotFound" {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("%w: %s (%s)", ErrRPCRequestFailed, rpcResp.Result.Error, rpcResp.Result.ErrorMessage)
	}

	var payments []PaymentEvent
	for _, entry := range rpcResp.Result.Transactions {
		// Rule 1: Must be validated by consensus
		if !entry.Validated {
			continue
		}

		// Rule 2: Must be a Payment transaction
		if entry.Tx.TransactionType != "Payment" {
			continue
		}

		// Rule 3: Transaction must have succeeded (tesSUCCESS)
		if entry.Meta.TransactionResult != "tesSUCCESS" {
			continue
		}

		// Rule 4: Must be destined for our monitored merchant account
		if entry.Tx.Destination != account {
			continue
		}

		// Rule 5: Extract delivered drops safely
		drops, err := parseDeliveredDrops(entry.Meta.DeliveredAmount)
		if err != nil {
			// Skip non-XRP (issued token) payments for now
			continue
		}

		payments = append(payments, PaymentEvent{
			TxHash:         entry.Tx.Hash,
			Sender:         entry.Tx.Account,
			Destination:    entry.Tx.Destination,
			DestinationTag: entry.Tx.DestinationTag,
			DeliveredDrops: drops,
			LedgerIndex:    entry.Tx.LedgerIndex,
			Validated:      entry.Validated,
		})
	}

	return payments, nil
}

// parseDeliveredDrops extracts the integer drops from the polymorphic delivered_amount field.
// In XRPL, XRP amounts are represented as strings (e.g. "10000000").
// Issued tokens are represented as JSON objects (e.g. {"currency": "USD", ...}).
func parseDeliveredDrops(raw interface{}) (domain.Drops, error) {
	switch v := raw.(type) {
	case string:
		dropsInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid drops string in tx meta: %w", err)
		}
		return domain.FromDrops(dropsInt)
	default:
		return 0, fmt.Errorf("non-XRP currency format")
	}
}
