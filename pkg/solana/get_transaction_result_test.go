package solana

import (
	"testing"

	"github.com/gagliardetto/solana-go/rpc"
)

func TestGetTransactionResult(t *testing.T) {
	node := rpc.New("")

	txHash := ""

	result, earnings, err := GetTransactionFeesAndEarnings(node, txHash, true)
	if err != nil {
		t.Fatalf("failed to get transaction result: %v", err)
	}
	t.Logf("transaction result: %v", result)
	t.Logf("earnings: %v", earnings)
}
