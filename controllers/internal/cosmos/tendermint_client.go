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

// TendermintStatus is the response from the /status RPC endpoint.
type TendermintStatus struct {
	JsonRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		NodeInfo struct {
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
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockHash     string    `json:"latest_block_hash"`
			LatestAppHash       string    `json:"latest_app_hash"`
			LatestBlockHeight   string    `json:"latest_block_height"`
			LatestBlockTime     time.Time `json:"latest_block_time"`
			EarliestBlockHash   string    `json:"earliest_block_hash"`
			EarliestAppHash     string    `json:"earliest_app_hash"`
			EarliestBlockHeight string    `json:"earliest_block_height"`
			EarliestBlockTime   time.Time `json:"earliest_block_time"`
			CatchingUp          bool      `json:"catching_up"`
		} `json:"sync_info"`
		ValidatorInfo struct {
			Address string `json:"address"`
			PubKey  struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"pub_key"`
			VotingPower string `json:"voting_power"`
		} `json:"validator_info"`
	} `json:"result"`
}

// LatestBlockHeight parses the latest block height string. If the string is malformed, returns 0.
func (status TendermintStatus) LatestBlockHeight() uint64 {
	h, _ := strconv.ParseUint(status.Result.SyncInfo.LatestBlockHeight, 10, 64)
	return h
}

// TendermintClient knows how to make requests to the Tendermint RPC endpoints.
// This package uses a custom client because 1) parsing JSON is simple and 2) we prevent any dependency on
// tendermint packages.
type TendermintClient struct {
	httpDo func(req *http.Request) (*http.Response, error)
}

func NewTendermintClient(client *http.Client) *TendermintClient {
	return &TendermintClient{httpDo: client.Do}
}

// Status finds the current Tendermint status.
func (client *TendermintClient) Status(ctx context.Context, rpcHost string) (TendermintStatus, error) {
	var status TendermintStatus
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
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return status, fmt.Errorf("malformed json: %w", err)
	}
	return status, err
}
