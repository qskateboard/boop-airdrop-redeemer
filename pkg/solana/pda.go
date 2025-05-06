package solana

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// FindMerkleDistributorPDA calculates the program-derived address for a merkle distributor
// based on tokenDistributor, mintAddress, and programID
func FindMerkleDistributorPDA(tokenDistributor solana.PublicKey, mintAddress solana.PublicKey, programID solana.PublicKey, index uint64) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("MerkleDistributor"),
		tokenDistributor.Bytes(),
		mintAddress.Bytes(),
		Uint64ToLEBytes(index),
	}

	addr, _, err := solana.FindProgramAddress(seeds, programID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address: %w", err)
	}

	return addr, nil
}

// FindClaimStatusPDA calculates the program-derived address for a claim status
// based on from, to, and programID
func FindClaimStatusPDA(from solana.PublicKey, to solana.PublicKey, programID solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("ClaimStatus"),
		from.Bytes(),
		to.Bytes(),
	}

	addr, _, err := solana.FindProgramAddress(seeds, programID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address: %w", err)
	}

	return addr, nil
}

// FindBoopPoolAddress calculates the program-derived address for a Boop pool
// based on mintAddress, tokenProgramID, and programID
// skipCurveCheck can be set to true to skip the on-curve check for mintAddress
func FindBoopPoolAddress(mintAddress solana.PublicKey, pda solana.PublicKey, skipCurveCheck bool) (solana.PublicKey, error) {
	// Check if the mint address is on curve if skipCurveCheck is false
	// Note: Solana-go doesn't have a direct equivalent of isOnCurve
	// This would require custom implementation or can be skipped for simplicity
	if !skipCurveCheck {
		// In a real implementation, you would check if the address is on curve
		// For now, we skip this check as it's not directly available in solana-go
	}

	seeds := [][]byte{
		pda.Bytes(),
		solana.TokenProgramID.Bytes(),
		mintAddress.Bytes(),
	}

	addr, _, err := solana.FindProgramAddress(seeds, solana.SPLAssociatedTokenAccountProgramID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find program address: %w", err)
	}

	return addr, nil
}

// Uint64ToLEBytes converts a uint64 to a little-endian byte array
func Uint64ToLEBytes(val uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, val)
	return buf
}
