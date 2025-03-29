package main

import (
	"regexp"
	"testing"
)

func TestNamespaceRegex(t *testing.T) {
	regex := regexp.MustCompile(`^cosmos-(fullnode|sentry)-[a-zA-Z0-9]+-(?:mainnet|testnet)$`)

	testCases := []struct {
		name      string
		namespace string
		expected  bool
	}{
		{"Valid fullnode mainnet", "cosmos-fullnode-osmosis-mainnet", true},
		{"Valid sentry testnet", "cosmos-sentry-axelar-testnet", true},
		{"Invalid prefix", "invalid-fullnode-kava-mainnet", false},
		{"Invalid suffix", "cosmos-fullnode-sei-devnet", false},
		{"Valid with numbers", "cosmos-fullnode-chain123-mainnet", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := regex.MatchString(tc.namespace)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for namespace %s", tc.expected, result, tc.namespace)
			}
		})
	}
}
