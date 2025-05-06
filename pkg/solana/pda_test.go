package solana

import (
	"fmt"
	"testing"

	sln "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
)

func TestFindMerkleDistributorPDA(t *testing.T) {
	// Define test inputs
	tokenDistributor := sln.MustPublicKeyFromBase58("J7cV46t2BLkoHWvmrcG1nK3wgB2D1EmHLko29bEDbnpV")
	mintAddress := sln.MustPublicKeyFromBase58("BuNonfvszzm6dJuzigNbde7qGNmcSYxT64erw3Wboop")
	programID := sln.MustPublicKeyFromBase58("boopEtkTLx8x8moK7mMBQZUfzaEiA96Qn7gQeNdcQMg")
	index := uint64(0)

	// Call the function
	pda, err := FindMerkleDistributorPDA(tokenDistributor, mintAddress, programID, index)

	fmt.Println("PDA:", pda.String())

	// Verify results
	assert.NoError(t, err)
	assert.NotEqual(t, sln.PublicKey{}, pda)

	// Test with a different index
	pda2, err := FindMerkleDistributorPDA(tokenDistributor, mintAddress, programID, 1)

	fmt.Println("PDA2:", pda2.String())
	assert.NoError(t, err)
	assert.NotEqual(t, pda, pda2, "PDAs with different indices should be different")

	// Verify deterministic behavior
	pdaDuplicate, err := FindMerkleDistributorPDA(tokenDistributor, mintAddress, programID, index)
	assert.NoError(t, err)
	assert.Equal(t, pda, pdaDuplicate, "PDAs with same inputs should be equal")

	// Test Uint64ToLEBytes
	bytes := Uint64ToLEBytes(42)
	assert.Equal(t, 8, len(bytes), "Byte array should be 8 bytes long")
	assert.Equal(t, byte(42), bytes[0], "First byte should be 42")
	assert.Equal(t, byte(0), bytes[1], "Second byte should be 0")

	wallet := sln.MustPublicKeyFromBase58("SkatebLAUZ9cmbayrLE3wWao3VuFsb1eGE3R7mCs2X2")
	differentWallet := sln.MustPublicKeyFromBase58("EeNF8G475Y7NGYJasMiB3c1u51JfzJKKYqzXmvTb3GTf")

	// Test FindClaimStatusPDA
	claimStatus, err := FindClaimStatusPDA(wallet, pda, programID)
	assert.NoError(t, err)
	assert.NotEqual(t, sln.PublicKey{}, claimStatus)
	fmt.Println("ClaimStatus:", claimStatus.String())

	// Test FindClaimStatusPDA with different inputs
	claimStatus2, err := FindClaimStatusPDA(differentWallet, pda, programID)
	assert.NoError(t, err)
	assert.NotEqual(t, claimStatus, claimStatus2, "Claim status PDAs with different inputs should be different")

	fmt.Println("ClaimStatus2:", claimStatus2.String())
	// Test deterministic behavior
	claimStatusDuplicate, err := FindClaimStatusPDA(wallet, pda, programID)
	assert.NoError(t, err)
	assert.Equal(t, claimStatus, claimStatusDuplicate, "Claim status PDAs with same inputs should be equal")

	fmt.Println("ClaimStatusDuplicate:", claimStatusDuplicate.String())
}

func TestFindBoopPoolAddress(t *testing.T) {
	// Define test inputs
	mintAddress := sln.MustPublicKeyFromBase58("BuNonfvszzm6dJuzigNbde7qGNmcSYxT64erw3Wboop")
	pda := sln.MustPublicKeyFromBase58("7EJfcAv4EkAxRtg9QG8xRHWKdmg74BS4JyckKqXBuriw")

	// Call the function
	boopPool, err := FindBoopPoolAddress(mintAddress, pda, true)

	// Verify results
	assert.NoError(t, err)
	assert.NotEqual(t, sln.PublicKey{}, boopPool)
	fmt.Println("Boop Pool Address:", boopPool.String())

	// Test with different values
	differentPda := sln.MustPublicKeyFromBase58("J7cV46t2BLkoHWvmrcG1nK3wgB2D1EmHLko29bEDbnpV")
	boopPool2, err := FindBoopPoolAddress(mintAddress, differentPda, true)

	assert.NoError(t, err)
	assert.NotEqual(t, boopPool, boopPool2, "Boop pool addresses with different PDAs should be different")
	fmt.Println("Boop Pool Address 2:", boopPool2.String())

	// Verify deterministic behavior
	boopPoolDuplicate, err := FindBoopPoolAddress(mintAddress, pda, true)
	assert.NoError(t, err)
	assert.Equal(t, boopPool, boopPoolDuplicate, "Boop pool addresses with the same inputs should be equal")
}
