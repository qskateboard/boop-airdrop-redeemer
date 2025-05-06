package solana

import (
	"context"
	"encoding/json"
	"strconv"

	solana_go "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func GetTransactionFeesAndEarnings(node *rpc.Client, txHash string, checkEarnings bool) (uint64, uint64, error) {
	maxSupportedTransactionVersion := uint64(0)

	var swapTxResult *rpc.GetParsedTransactionResult
	var err error

	swapTxResult, err = node.GetParsedTransaction(
		context.Background(),
		solana_go.MustSignatureFromBase58(txHash),
		&rpc.GetParsedTransactionOpts{
			Commitment:                     "confirmed",
			MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
		},
	)

	if err != nil {
		return 0, 0, err
	}

	fee := swapTxResult.Meta.Fee

	var earnings uint64 = 0

	if checkEarnings {
		// Find the signer's wallet
		var signerWallet solana_go.PublicKey
		for _, account := range swapTxResult.Transaction.Message.AccountKeys {
			if account.Signer {
				signerWallet = account.PublicKey
				break
			}
		}

		// Go through inner instructions to find WSOL transfers to the signer
		for _, innerInsts := range swapTxResult.Meta.InnerInstructions {
			for _, inst := range innerInsts.Instructions {
				// Look for spl-token program instructions
				if inst.Program != "spl-token" {
					continue
				}

				// Use json to extract data since AsMap is not available
				var parsedData map[string]interface{}
				if inst.Parsed != nil {
					jsonData, err := json.Marshal(inst.Parsed)
					if err != nil {
						continue
					}

					if err := json.Unmarshal(jsonData, &parsedData); err != nil {
						continue
					}
				} else {
					continue
				}

				// Check instruction type
				instructionType, ok := parsedData["type"].(string)
				if !ok || instructionType != "transferChecked" {
					continue
				}

				// Extract info
				info, ok := parsedData["info"].(map[string]interface{})
				if !ok {
					continue
				}

				// Check if mint is WSOL
				mint, ok := info["mint"].(string)
				if !ok || mint != "So11111111111111111111111111111111111111112" {
					continue
				}

				// Check destination account
				destination, ok := info["destination"].(string)
				if !ok {
					continue
				}

				// Check token amount
				tokenAmount, ok := info["tokenAmount"].(map[string]interface{})
				if !ok {
					continue
				}

				amount, ok := tokenAmount["amount"].(string)
				if !ok {
					continue
				}

				// Check if destination is associated with the signer
				isForSigner := false

				// Direct transfer to signer
				if destination == signerWallet.String() {
					isForSigner = true
				} else {
					// Check if the destination is an associated token account for the signer
					// that was created in this transaction
					for _, mainInst := range swapTxResult.Transaction.Message.Instructions {
						if mainInst.Program == "spl-associated-token-account" || mainInst.Program == "spl-token" {
							var mainParsedData map[string]interface{}
							if mainInst.Parsed != nil {
								jsonData, err := json.Marshal(mainInst.Parsed)
								if err != nil {
									continue
								}

								if err := json.Unmarshal(jsonData, &mainParsedData); err != nil {
									continue
								}
							} else {
								continue
							}

							instType, ok := mainParsedData["type"].(string)
							if !ok {
								continue
							}

							if instType == "createIdempotent" || instType == "initializeAccount3" {
								instInfo, ok := mainParsedData["info"].(map[string]interface{})
								if !ok {
									continue
								}

								account, ok1 := instInfo["account"].(string)
								wallet, ok2 := instInfo["wallet"].(string)

								if ok1 && ok2 && account == destination && wallet == signerWallet.String() {
									isForSigner = true
									break
								}
							}

							if instType == "closeAccount" {
								instInfo, ok := mainParsedData["info"].(map[string]interface{})
								if !ok {
									continue
								}

								account, ok1 := instInfo["account"].(string)
								dest, ok2 := instInfo["destination"].(string)

								if ok1 && ok2 && account == destination && dest == signerWallet.String() {
									isForSigner = true
									break
								}
							}
						}
					}
				}

				if isForSigner {
					// Convert amount string to uint64
					amountU64, err := strconv.ParseUint(amount, 10, 64)
					if err == nil {
						earnings += amountU64
					}
				}
			}
		}
	}

	return fee, earnings, nil
}
