package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/cosmos"
	"github.com/stretchr/testify/require"
)

type mockClient func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error)

func (fn mockClient) Status(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
	return fn(ctx, rpcHost)
}

var nopLogger = logr.Discard()

func TestHandler(t *testing.T) {
	var (
		stubReq = httptest.NewRequest("GET", "/", nil)
	)
	const testRpc = "http://my-rpc:25567"

	t.Run("happy path", func(t *testing.T) {
		client := mockClient(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			require.NotNil(t, ctx)
			require.Equal(t, testRpc, rpcHost)
			return cosmos.TendermintStatus{}, nil
		})

		h := NewTendermint(nopLogger, client, testRpc, 10*time.Second)
		w := httptest.NewRecorder()
		h.Handle(w, stubReq)

		require.Equal(t, http.StatusOK, w.Code)
		var got healthResponse
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)

		want := healthResponse{
			Address: testRpc,
			InSync:  true,
		}
		require.Equal(t, want, got)
	})

	t.Run("still catching up", func(t *testing.T) {
		client := mockClient(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			var stub cosmos.TendermintStatus
			stub.Result.SyncInfo.CatchingUp = true
			return stub, nil
		})

		h := NewTendermint(nopLogger, client, testRpc, 10*time.Second)
		w := httptest.NewRecorder()
		h.Handle(w, stubReq)

		require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var got healthResponse
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)

		want := healthResponse{
			Address: testRpc,
			InSync:  false,
		}
		require.Equal(t, want, got)
	})

	t.Run("status error", func(t *testing.T) {
		client := mockClient(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			return cosmos.TendermintStatus{}, errors.New("boom")
		})

		h := NewTendermint(nopLogger, client, testRpc, 10*time.Second)
		w := httptest.NewRecorder()
		h.Handle(w, stubReq)

		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		var got healthResponse
		err := json.NewDecoder(w.Body).Decode(&got)
		require.NoError(t, err)

		want := healthResponse{
			Address: testRpc,
			Error:   "boom",
		}
		require.Equal(t, want, got)
	})

	t.Run("times out", func(t *testing.T) {
		var gotCtx context.Context
		client := mockClient(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			gotCtx = ctx
			return cosmos.TendermintStatus{}, nil
		})

		h := NewTendermint(nopLogger, client, testRpc, time.Nanosecond)
		w := httptest.NewRecorder()
		h.Handle(w, stubReq)

		select {
		case <-gotCtx.Done():
		// Test passes.
		case <-time.After(3 * time.Second):
			require.Fail(t, "context did not time out")
		}
	})
}
