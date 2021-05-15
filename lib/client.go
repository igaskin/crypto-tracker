package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ExplorerClientInterface interface {
	// GetAccount request
	GetAccount(ctx context.Context, opts *GetAccountOpts) (*GetAccountResponse, error)

	// GetAccountTransaction request
	GetAccountTransaction(ctx context.Context, opts *GetAccountTransactionOpts) (*GetAccountTransactionResponse, error)
}

type ExplorerClient struct {
	// API endpoint
	// default: https://crypto.org/explorer/api/v1/
	Server string

	Client *http.Client
}

// Creates a new ExplorerClient, with reasonable defaults
func NewExplorerClient(server string) *ExplorerClient {
	explorerClient := ExplorerClient{
		Server: server,
	}
	// ensure the server URL always has a trailing slash
	if !strings.HasSuffix(explorerClient.Server, "/") {
		explorerClient.Server += "/"
	}
	// create httpClient, if not already present
	if explorerClient.Client == nil {
		explorerClient.Client = &http.Client{}
	}
	return &explorerClient
}

func (c *ExplorerClient) GetAccount(ctx context.Context, opts *GetAccountOpts) (*GetAccountResponse, error) {
	var err error

	serverURL, err := url.Parse(c.Server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/accounts/%s", opts.AccountID)
	if operationPath[0] == '/' {
		operationPath = operationPath[1:]
	}
	operationURL := url.URL{
		Path: operationPath,
	}

	queryURL := serverURL.ResolveReference(&operationURL)

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var account GetAccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}
	return &account, nil

}

type GetAccountOpts struct {
	AccountID string
}

type GetAccountResponse struct {
	Result Result `json:"result"`
}
type Balance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
type Bondedbalance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
type Totalrewards struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
type Totalbalance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
type Result struct {
	Type                string          `json:"type"`
	Name                string          `json:"name"`
	Address             string          `json:"address"`
	Balance             []Balance       `json:"balance"`
	Bondedbalance       []Bondedbalance `json:"bondedBalance"`
	Redelegatingbalance []interface{}   `json:"redelegatingBalance"`
	Unbondingbalance    []interface{}   `json:"unbondingBalance"`
	Totalrewards        []Totalrewards  `json:"totalRewards"`
	Commissions         []interface{}   `json:"commissions"`
	Totalbalance        []Totalbalance  `json:"totalBalance"`
}

type GetAccountTransactionOpts struct {
	// page=5&limit=8&order=height.desc
	Account string
	Page    int32
	limit   int32
	Order   string
}

type GetAccountTransactionResponse struct {
	Result     []TransactionResult `json:"result"`
	Pagination Pagination          `json:"pagination"`
}
type Fee struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
type Amount struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}
type Content struct {
	Name             string   `json:"name"`
	UUID             string   `json:"uuid"`
	Height           int      `json:"height"`
	Msgname          string   `json:"msgName"`
	Msgindex         int      `json:"msgIndex"`
	Delegatoraddress string   `json:"delegatorAddress"`
	Recipientaddress string   `json:"recipientAddress"`
	Amount           []Amount `json:"amount"`
	Txhash           string   `json:"txHash"`
	Version          int      `json:"version"`
	Validatoraddress string   `json:"validatorAddress"`
}

type Messages struct {
	Type    string  `json:"type"`
	Content Content `json:"content,omitempty"`
}

type TransactionResult struct {
	Account       string     `json:"account"`
	Blockheight   int        `json:"blockHeight"`
	Blockhash     string     `json:"blockHash"`
	Blocktime     time.Time  `json:"blockTime"`
	Hash          string     `json:"hash"`
	Messagetypes  []string   `json:"messageTypes"`
	Success       bool       `json:"success"`
	Code          int        `json:"code"`
	Log           string     `json:"log"`
	Fee           []Fee      `json:"fee"`
	Feepayer      string     `json:"feePayer"`
	Feegranter    string     `json:"feeGranter"`
	Gaswanted     int        `json:"gasWanted"`
	Gasused       int        `json:"gasUsed"`
	Memo          string     `json:"memo"`
	Timeoutheight int        `json:"timeoutHeight"`
	Messages      []Messages `json:"messages"`
}
type Pagination struct {
	TotalRecord int `json:"total_record"`
	TotalPage   int `json:"total_page"`
	CurrentPage int `json:"current_page"`
	Limit       int `json:"limit"`
}
