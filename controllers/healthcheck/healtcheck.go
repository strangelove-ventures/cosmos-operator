package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/cosmos"
)

// Statuser can query the Tendermint status endpoint.
type Statuser interface {
	Status(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error)
}

type healthResponse struct {
	Address string `json:"address"`
	InSync  bool   `json:"in_sync"`
	Error   string `json:"error,omitempty"`
}

// Tendermint checks the Tendermint status endpoint to determine if the node is in-sync or not.
type Tendermint struct {
	client  Statuser
	rpcHost string
	timeout time.Duration
}

func NewTendermint(client Statuser, rpcHost string, timeout time.Duration) *Tendermint {
	return &Tendermint{
		client:  client,
		rpcHost: rpcHost,
		timeout: timeout,
	}
}

// Handle implements http.HandlerFunc signature.
func (h Tendermint) Handle(w http.ResponseWriter, r *http.Request) {
	var resp healthResponse
	resp.Address = h.rpcHost

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	status, err := h.client.Status(ctx, h.rpcHost)
	if err != nil {
		resp.Error = err.Error()
		h.writeResponse(http.StatusServiceUnavailable, w, resp)
		return
	}

	resp.InSync = !status.Result.SyncInfo.CatchingUp
	if !resp.InSync {
		h.writeResponse(http.StatusUnprocessableEntity, w, resp)
		return
	}

	h.writeResponse(http.StatusOK, w, resp)
}

func (h Tendermint) writeResponse(code int, w http.ResponseWriter, resp healthResponse) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// This should never happen.
		panic(fmt.Errorf("json encode response: %w", err))
	}
}
