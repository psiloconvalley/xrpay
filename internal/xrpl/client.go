package xrpl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

var (
	ErrRPCRequestFailed  = errors.New("xrpl rpc request failed")
	ErrAccountNotFound   = errors.New("xrpl account not found (unfunded)")
	ErrMalformedResponse = errors.New("malformed response from xrpl node")
)

type Client struct {
	rpcURL     string
	httpClient *http.Client
}

func NewClient(rpcURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

type AccountPayment struct {
	TxHash         string
	LedgerIndex    int64
	DeliveredDrops domain.Drops
	DestinationTag uint32
	Account        string
	Destination    string
	Timestamp      time.Time
}

func (c *Client) GetAccountPayments(ctx context.Context, account string, minLedger int64, maxLedger int64) ([]AccountPayment, int64, error) {
	params := AccountTxParams{
		Account: account,
		Forward: true,
	}

	if minLedger > 0 {
		params.LedgerIndexMin = minLedger
		params.LedgerIndexMax = -1
	}

	reqBody := RPCRequest{
		Method: "account_tx",
		Params: []interface{}{params},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed marshalling rpc request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("failed creating http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrRPCRequestFailed, err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed reading response body: %w", err)
	}

	var rpcResp AccountTxResponse
	if err := json.Unmarshal(bodyBytes, &rpcResp); err != nil {
		return nil, 0, fmt.Errorf("%w: %s", ErrMalformedResponse, err.Error())
	}

	if rpcResp.Result.Status != "success" && rpcResp.Result.Error != "" {
		if rpcResp.Result.Error == "actNotFound" {
			return nil, 0, ErrAccountNotFound
		}
		return nil, 0, fmt.Errorf("%w: [%s] %s", ErrRPCRequestFailed, rpcResp.Result.Error, rpcResp.Result.ErrorMessage)
	}

	payments := make([]AccountPayment, 0, len(rpcResp.Result.Transactions))
	latestLedger := minLedger

	for _, txItem := range rpcResp.Result.Transactions {
		if txItem.Tx.TransactionType != "Payment" {
			continue
		}
		if !txItem.Validated {
			continue
		}
		if txItem.Meta.TransactionResult != "tesSUCCESS" {
			continue
		}
		if txItem.Tx.Destination != account {
			continue
		}

		ledgerIdx := int64(txItem.Tx.LedgerIndex)
		if ledgerIdx == 0 {
			ledgerIdx = int64(txItem.Tx.InLedger)
		}
		if ledgerIdx == 0 {
			ledgerIdx = int64(txItem.LedgerIndex)
		}

		if ledgerIdx > latestLedger {
			latestLedger = ledgerIdx
		}

		// Security: Check DeliveredAmount to avoid partial payment exploits
		var dropsStr string
		if txItem.Meta.DeliveredAmount != nil {
			if s, ok := txItem.Meta.DeliveredAmount.(string); ok {
				dropsStr = s
			}
		}

		if dropsStr == "" {
			continue
		}

		dropsInt, err := strconv.ParseInt(dropsStr, 10, 64)
		if err != nil {
			continue
		}

		paymentTime := time.Unix(txItem.Tx.Date+946684800, 0).UTC()

		payments = append(payments, AccountPayment{
			TxHash:         txItem.Tx.Hash,
			LedgerIndex:    ledgerIdx,
			DeliveredDrops: domain.Drops(dropsInt),
			DestinationTag: txItem.Tx.DestinationTag,
			Account:        txItem.Tx.Account,
			Destination:    txItem.Tx.Destination,
			Timestamp:      paymentTime,
		})
	}

	return payments, latestLedger, nil
}
