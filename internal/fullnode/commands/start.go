package commands

import (
	"fmt"
	"strings"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

const startScriptWrapper = `
%s
`

func StartCmdAndArgs(crd *cosmosv1.CosmosFullNode, homeDir string) (string, []string) {
	if crd.Spec.ChainSpec.StartScript == nil {
		var (
			binary             = crd.Spec.ChainSpec.Binary
			args               = StartCommandArgs(crd, homeDir)
			privvalSleep int32 = 10
		)
		if v := crd.Spec.ChainSpec.PrivvalSleepSeconds; v != nil {
			privvalSleep = *v
		}

		if crd.Spec.Type == cosmosv1.Sentry && privvalSleep > 0 {
			shellBody := fmt.Sprintf(`sleep %d
%s %s`, privvalSleep, binary, strings.Join(args, " "))
			return "sh", []string{"-c", shellBody}
		}

		return binary, args
	} else {
		args := []string{"-c"}
		args = append(args, fmt.Sprintf(startScriptWrapper, *crd.Spec.ChainSpec.StartScript))
		return "sh", args
	}
}

func StartCommandArgs(crd *cosmosv1.CosmosFullNode, homeDir string) []string {
	args := []string{"start", "--home", homeDir}
	cfg := crd.Spec.ChainSpec
	if cfg.SkipInvariants {
		args = append(args, "--x-crisis-skip-assert-invariants")
	}
	if lvl := cfg.LogLevel; lvl != nil {
		args = append(args, "--log_level", *lvl)
	}
	if format := cfg.LogFormat; format != nil {
		args = append(args, "--log_format", *format)
	}
	if len(crd.Spec.ChainSpec.AdditionalStartArgs) > 0 {
		args = append(args, crd.Spec.ChainSpec.AdditionalStartArgs...)
	}
	return args
}
