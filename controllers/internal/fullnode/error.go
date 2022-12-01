package fullnode

import (
	"github.com/go-logr/logr"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

func RecordError(log logr.Logger, crd *cosmosv1.CosmosFullNode, err error) error {
	if err == nil {
		return nil
	}

}
