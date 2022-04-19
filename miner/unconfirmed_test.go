package miner

import (
	"testing"

	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/core/types"
)

// noopChainRetriever is an implementation of headerRetriever that always
// returns nil for any requested headers.
type noopChainRetriever struct{}

func (r *noopChainRetriever) GetHeaderByNumber(number uint64) *types.Header {
	return nil
}
func (r *noopChainRetriever) GetBlockByNumber(number uint64) *types.Block {
	return nil
}

// Tests that inserting blocks into the unconfirmed set accumulates them until
// the desired depth is reached, after which they begin to be dropped.
func TestUnconfirmedInsertBounds(t *testing.T) {
	limit := uint(10)

	pool := newUnconfirmedBlocks(new(noopChainRetriever), limit)
	for depth := uint64(0); depth < 2*uint64(limit); depth++ {
		// Insert multiple blocks for the same level just to stress it
		for i := 0; i < int(depth); i++ {
			pool.Insert(depth, common.Hash([32]byte{byte(depth), byte(i)}))
		}
		// Validate that no blocks below the depth allowance are left in
		pool.blocks.Do(func(block interface{}) {
			if block := block.(*unconfirmedBlock); block.index+uint64(limit) <= depth {
				t.Errorf("depth %d: block %x not dropped", depth, block.hash)
			}
		})
	}
}

// Tests that shifting blocks out of the unconfirmed set works both for normal
// cases as well as for corner cases such as empty sets, empty shifts or full
// shifts.
func TestUnconfirmedShifts(t *testing.T) {
	// Create a pool with a few blocks on various depths
	limit, start := uint(10), uint64(25)

	pool := newUnconfirmedBlocks(new(noopChainRetriever), limit)
	for depth := start; depth < start+uint64(limit); depth++ {
		pool.Insert(depth, common.Hash([32]byte{byte(depth)}))
	}
	// Try to shift below the limit and ensure no blocks are dropped
	pool.Shift(start + uint64(limit) - 1)
	if n := pool.blocks.Len(); n != int(limit) {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, limit)
	}
	// Try to shift half the blocks out and verify remainder
	pool.Shift(start + uint64(limit) - 1 + uint64(limit/2))
	if n := pool.blocks.Len(); n != int(limit)/2 {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, limit/2)
	}
	// Try to shift all the remaining blocks out and verify emptyness
	pool.Shift(start + 2*uint64(limit))
	if n := pool.blocks.Len(); n != 0 {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, 0)
	}
	// Try to shift out from the empty set and make sure it doesn't break
	pool.Shift(start + 3*uint64(limit))
	if n := pool.blocks.Len(); n != 0 {
		t.Errorf("unconfirmed count mismatch: have %d, want %d", n, 0)
	}
}
