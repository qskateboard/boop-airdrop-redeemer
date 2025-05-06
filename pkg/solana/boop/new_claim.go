package boop

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// NewClaim is the instruction data for claiming tokens
type NewClaim struct {
	AmountUnlocked uint64
	AmountLocked   uint64
	Proof          [][32]uint8

	accounts solana.AccountMetaSlice
}

var ProgramID solana.PublicKey = solana.MustPublicKeyFromBase58("boopEtkTLx8x8moK7mMBQZUfzaEiA96Qn7gQeNdcQMg")

func (inst *NewClaim) ProgramID() solana.PublicKey {
	return ProgramID
}

func (inst *NewClaim) Accounts() []*solana.AccountMeta {
	return inst.accounts
}

func (inst *NewClaim) GetAccounts() solana.AccountMetaSlice {
	return inst.accounts
}

func (inst *NewClaim) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

func (inst *NewClaim) Build() solana.Instruction {
	return inst
}

func (inst *NewClaim) MarshalWithEncoder(encoder *bin.Encoder) error {
	// Write the instruction discriminator (0)
	discriminator := sha256.Sum256([]byte("global:new_claim"))

	discriminatorBytes := discriminator[:8]

	if err := encoder.WriteBytes(discriminatorBytes[:], false); err != nil {
		return err
	}

	// Set the AmountUnlocked field
	if err := encoder.Encode(inst.AmountUnlocked); err != nil {
		return fmt.Errorf("unable to encode AmountUnlocked: %w", err)
	}

	// Set the AmountLocked field
	if err := encoder.Encode(inst.AmountLocked); err != nil {
		return fmt.Errorf("unable to encode AmountLocked: %w", err)
	}

	// Set the Proof field
	if err := encoder.Encode(inst.Proof); err != nil {
		return fmt.Errorf("unable to encode Proof: %w", err)
	}

	return nil
}

func (inst *NewClaim) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	// Skip the instruction discriminator
	_, err := decoder.ReadUint8()
	if err != nil {
		return fmt.Errorf("unable to decode instruction discriminator: %w", err)
	}

	// Set the AmountUnlocked field
	err = decoder.Decode(&inst.AmountUnlocked)
	if err != nil {
		return fmt.Errorf("unable to decode AmountUnlocked: %w", err)
	}

	// Set the AmountLocked field
	err = decoder.Decode(&inst.AmountLocked)
	if err != nil {
		return fmt.Errorf("unable to decode AmountLocked: %w", err)
	}

	// Set the Proof field
	err = decoder.Decode(&inst.Proof)
	if err != nil {
		return fmt.Errorf("unable to decode Proof: %w", err)
	}

	return nil
}

// NewNewClaimInstructionBuilder creates a new instruction builder for the NewClaim instruction
func NewNewClaimInstructionBuilder(
	// Parameters
	amountUnlocked uint64,
	amountLocked uint64,
	proof [][32]uint8,

	// Accounts
	distributor solana.PublicKey,
	claimStatus solana.PublicKey,
	from solana.PublicKey,
	to solana.PublicKey,
	claimant solana.PublicKey,
) *NewClaim {
	nd := &NewClaim{
		AmountUnlocked: amountUnlocked,
		AmountLocked:   amountLocked,
		Proof:          proof,
		accounts:       make(solana.AccountMetaSlice, 7),
	}
	nd.accounts[0] = solana.Meta(distributor).WRITE()
	nd.accounts[1] = solana.Meta(claimStatus).WRITE()
	nd.accounts[2] = solana.Meta(from).WRITE()
	nd.accounts[3] = solana.Meta(to).WRITE()
	nd.accounts[4] = solana.Meta(claimant).WRITE().SIGNER()
	nd.accounts[5] = solana.Meta(solana.TokenProgramID)
	nd.accounts[6] = solana.Meta(solana.SystemProgramID)

	return nd
}
