package cosmos

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTendermintStatus_LatestBlockHeight(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		Height string
		Want   uint64
	}{
		{"", 0},
		{"huh", 0},
		{"-1", 0},
		{"1", 1},
		{"1234567", 1234567},
	} {
		var status TendermintStatus
		status.Result.SyncInfo.LatestBlockHeight = tt.Height

		require.Equal(t, tt.Want, status.LatestBlockHeight(), tt)
	}
}

func TestTendermintClient_Status(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		// This context ensures we're not comparing instances of context.Background().
		cctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client := NewTendermintClient(http.DefaultClient)
		require.NotNil(t, client.httpDo)

		client.httpDo = func(req *http.Request) (*http.Response, error) {
			require.Same(t, cctx, req.Context())
			require.Equal(t, "GET", req.Method)
			require.Equal(t, "http://10.2.3.4:26657/status", req.URL.String())

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(statusResponseFixture)),
			}, nil
		}

		got, err := client.Status(cctx, "http://10.2.3.4:26657")
		require.NoError(t, err)
		require.Equal(t, "cosmoshub-testnet-fullnode-0", got.Result.NodeInfo.Moniker)
		require.Equal(t, false, got.Result.SyncInfo.CatchingUp)
		require.Equal(t, "13348657", got.Result.SyncInfo.LatestBlockHeight)
		require.Equal(t, "9034670", got.Result.SyncInfo.EarliestBlockHeight)
	})

	t.Run("non 200 response", func(t *testing.T) {
		client := NewTendermintClient(http.DefaultClient)
		client.httpDo = func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Status:     "internal server error",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}

		_, err := client.Status(context.Background(), "http://10.2.3.4:26657")
		require.Error(t, err)
		require.EqualError(t, err, "internal server error")
	})

	t.Run("http error", func(t *testing.T) {
		client := NewTendermintClient(http.DefaultClient)
		client.httpDo = func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		}

		_, err := client.Status(context.Background(), "http://10.2.3.4:26657")
		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}

const statusResponseFixture = `{
  "jsonrpc": "2.0",
  "id": -1,
  "result": {
    "node_info": {
      "protocol_version": {
        "p2p": "8",
        "block": "11",
        "app": "0"
      },
      "id": "f5fe383c6338c14f94319a96813ea77df1ab9060",
      "listen_addr": "tcp://0.0.0.0:26656",
      "network": "theta-testnet-001",
      "version": "v0.34.21",
      "channels": "40202122233038606100",
      "moniker": "cosmoshub-testnet-fullnode-0",
      "other": {
        "tx_index": "on",
        "rpc_address": "tcp://0.0.0.0:26657"
      }
    },
    "sync_info": {
      "latest_block_hash": "B6672ADAAC1B94DA33C98A78451E78A6A1E53616DFFFEAAA89C138A59FDA0C5B",
      "latest_app_hash": "7839C1CC04613DFE51EB2218B50FA824B4A32EC5F04B12C8DE8B1A6A815BD406",
      "latest_block_height": "13348657",
      "latest_block_time": "2022-11-28T21:58:24.609825528Z",
      "earliest_block_hash": "8C0507EBF07B2EDAE96FB598856D732D2DF30506E719E0BB6C3F7230DD14F7EF",
      "earliest_app_hash": "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855",
      "earliest_block_height": "9034670",
      "earliest_block_time": "2019-12-11T16:11:34Z",
      "catching_up": false
    },
    "validator_info": {
      "address": "6F741B2D3789866798F58E25B412972E641EC035",
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value": "8YZP1BmtRJDd0uVKxQbxxDPZ3KjWGxX7EcMWYGbrUKk="
      },
      "voting_power": "0"
    }
  }
}
`
