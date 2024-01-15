package cosmos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type ValidatorInfo struct {
	Address string `json:"address"`
	PubKey  struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"pub_key"`
	VotingPower string `json:"voting_power"`
}

type NodeInfo struct {
	ProtocolVersion struct {
		P2P   string `json:"p2p"`
		Block string `json:"block"`
		App   string `json:"app"`
	} `json:"protocol_version"`
	ID         string `json:"id"`
	ListenAddr string `json:"listen_addr"`
	Network    string `json:"network"`
	Version    string `json:"version"`
	Channels   string `json:"channels"`
	Moniker    string `json:"moniker"`
	Other      struct {
		TxIndex    string `json:"tx_index"`
		RPCAddress string `json:"rpc_address"`
	} `json:"other"`
}

type SyncInfo struct {
	LatestBlockHash     string    `json:"latest_block_hash"`
	LatestAppHash       string    `json:"latest_app_hash"`
	LatestBlockHeight   string    `json:"latest_block_height"`
	LatestBlockTime     time.Time `json:"latest_block_time"`
	EarliestBlockHash   string    `json:"earliest_block_hash"`
	EarliestAppHash     string    `json:"earliest_app_hash"`
	EarliestBlockHeight string    `json:"earliest_block_height"`
	EarliestBlockTime   time.Time `json:"earliest_block_time"`
	CatchingUp          bool      `json:"catching_up"`
}

// rpcCometStatusResponse is the union of possible server response
type rpcCometStatusResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		NodeInfo      *NodeInfo      `json:"node_info"`
		SyncInfo      *SyncInfo      `json:"sync_info"`
		ValidatorInfo *ValidatorInfo `json:"validator_info"`
	} `json:"result"`
	NodeInfo      *NodeInfo      `json:"node_info"`
	SyncInfo      *SyncInfo      `json:"sync_info"`
	ValidatorInfo *ValidatorInfo `json:"validator_info"`
}

// CometStatus is the common response from the /status RPC endpoint.
type CometStatus struct {
	JSONRPC string
	ID      int
	Result  struct {
		NodeInfo      NodeInfo
		SyncInfo      SyncInfo
		ValidatorInfo ValidatorInfo
	}
}

// LatestBlockHeight parses the latest block height string. If the string is malformed, returns 0.
func (status CometStatus) LatestBlockHeight() uint64 {
	h, _ := strconv.ParseUint(status.Result.SyncInfo.LatestBlockHeight, 10, 64)
	return h
}

// CometClient knows how to make requests to the CometBFT (formerly Comet) RPC endpoints.
// This package uses a custom client because 1) parsing JSON is simple and 2) we prevent any dependency on
// CometBFT packages.
type CometClient struct {
	httpDo func(req *http.Request) (*http.Response, error)
}

func NewCometClient(client *http.Client) *CometClient {
	return &CometClient{httpDo: client.Do}
}

// Status finds the latest status.
func (client *CometClient) Status(ctx context.Context, rpcHost string) (CometStatus, error) {
	var status CometStatus
	u, err := url.ParseRequestURI(rpcHost)
	if err != nil {
		return status, fmt.Errorf("malformed host: %w", err)
	}
	u.Path = "status"
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return status, fmt.Errorf("malformed request: %w", err)
	}
	req = req.WithContext(ctx)
	resp, err := client.httpDo(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status, errors.New(resp.Status)
	}
	var rpcStatusResponse rpcCometStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&rpcStatusResponse)
	if err != nil {
		return status, fmt.Errorf("malformed json: %w", err)
	}
	if rpcStatusResponse.ValidatorInfo != nil {
		status.Result.ValidatorInfo = *rpcStatusResponse.ValidatorInfo
		status.Result.SyncInfo = *rpcStatusResponse.SyncInfo
		status.Result.NodeInfo = *rpcStatusResponse.NodeInfo
	} else {
		status.Result.ValidatorInfo = *rpcStatusResponse.Result.ValidatorInfo
		status.Result.SyncInfo = *rpcStatusResponse.Result.SyncInfo
		status.Result.NodeInfo = *rpcStatusResponse.Result.NodeInfo
	}
	return status, err
}
