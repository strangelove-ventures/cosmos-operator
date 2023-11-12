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

func TestCometStatus_LatestBlockHeight(t *testing.T) {
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
		var status CometStatus
		status.Result.SyncInfo.LatestBlockHeight = tt.Height

		require.Equal(t, tt.Want, status.LatestBlockHeight(), tt)
	}
}

func TestCometClient_Status(t *testing.T) {
	t.Parallel()

	t.Run("when there are no errors", func(t *testing.T) {
		t.Run("given common response it returns the status", func(t *testing.T) {
			// This context ensures we're not comparing instances of context.Background().
			cctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			client := NewCometClient(http.DefaultClient)
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

		t.Run("given SEI response it returns the status", func(t *testing.T) {
			// This context ensures we're not comparing instances of context.Background().
			cctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			client := NewCometClient(http.DefaultClient)
			require.NotNil(t, client.httpDo)

			client.httpDo = func(req *http.Request) (*http.Response, error) {
				require.Same(t, cctx, req.Context())
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "http://10.2.3.4:26657/status", req.URL.String())

				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(seiStatusResponseFixture)),
				}, nil
			}

			got, err := client.Status(cctx, "http://10.2.3.4:26657")
			require.NoError(t, err)
			require.Equal(t, "hello-sei-relayer", got.Result.NodeInfo.Moniker)
			require.Equal(t, false, got.Result.SyncInfo.CatchingUp)
			require.Equal(t, "37909189", got.Result.SyncInfo.LatestBlockHeight)
			require.Equal(t, "33517999", got.Result.SyncInfo.EarliestBlockHeight)
		})
	})

	t.Run("when there is an error", func(t *testing.T) {
		t.Run("non 200 response", func(t *testing.T) {
			client := NewCometClient(http.DefaultClient)
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
			client := NewCometClient(http.DefaultClient)
			client.httpDo = func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}

			_, err := client.Status(context.Background(), "http://10.2.3.4:26657")
			require.Error(t, err)
			require.EqualError(t, err, "boom")
		})
	})
}

const statusResponseFixture = `
{
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
const seiStatusResponseFixture = `
{
  "node_info": {
    "protocol_version": {
      "p2p": "8",
      "block": "11",
      "app": "0"
    },
    "id": "a08b049a9d1909252f96973c3038db88b4b12883",
    "listen_addr": "65.109.78.7:11956",
    "network": "pacific-1",
    "version": "0.35.0-unreleased",
    "channels": "40202122233038606162630070717273",
    "moniker": "hello-sei-relayer",
    "other": {
      "tx_index": "on",
      "rpc_address": "tcp://0.0.0.0:11957"
    }
  },
  "application_info": {
    "version": "9"
  },
  "sync_info": {
    "latest_block_hash": "24F4FEF7C5B704C99B3C36964ECCA855B9480E07F711B0A5FB2C348A4CAC5D48",
    "latest_app_hash": "65271A3CA49CDC29FDC3A33974C508626F47CA178BCEA9421275E752824BC107",
    "latest_block_height": "37909189",
    "latest_block_time": "2023-11-09T17:36:20.235115543Z",
    "earliest_block_hash": "0127F5D1CF7B53007180EB11052BB2B54D06C75EDE94E0F6686EA1988464B5B9",
    "earliest_app_hash": "643CB8B56EA1A5F58D12396D26D8D89C7F9601FC199C6ACF76569BFCEE8C548C",
    "earliest_block_height": "33517999",
    "earliest_block_time": "2023-10-20T18:56:19.244399909Z",
    "max_peer_block_height": "37909188",
    "catching_up": false,
    "total_synced_time": "0",
    "remaining_time": "0",
    "total_snapshots": "0",
    "chunk_process_avg_time": "0",
    "snapshot_height": "0",
    "snapshot_chunks_count": "0",
    "snapshot_chunks_total": "0",
    "backfilled_blocks": "0",
    "backfill_blocks_total": "0"
  },
  "validator_info": {
    "address": "FD7DCAA2E2C25770720E844E0FEA6D0004940B02",
    "pub_key": {
      "type": "tendermint/PubKeyEd25519",
      "value": "7znc+z+uh4QFjLHTQPgSrXZvUpFRNMM9hjQCXNZYtyA="
    },
    "voting_power": "0"
  }
}
`
