package solana

import (
	"context"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type BlockhashCacheStruct struct {
	mu        sync.Mutex
	blockhash solana.Hash
	Block     *rpc.LatestBlockhashResult
	expiry    time.Time
	ttl       time.Duration
}

var BlockhashCache = NewBlockhashCache(20 * time.Second)

func NewBlockhashCache(ttl time.Duration) *BlockhashCacheStruct {
	return &BlockhashCacheStruct{
		ttl: ttl,
	}
}

func (c *BlockhashCacheStruct) GetBlockhash(node *rpc.Client) (*BlockhashCacheStruct, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Now().Before(c.expiry) {
		return c, nil
	}
	block, err := node.GetLatestBlockhash(context.Background(), rpc.CommitmentConfirmed)
	if err != nil {
		return nil, err
	}

	c.blockhash = block.Value.Blockhash
	c.Block = block.Value
	c.expiry = time.Now().Add(c.ttl)

	return c, nil
}
