package core

import (
	"container/list"

	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/core/types"
	"github.com/ChainAAS/gendchain/ethdb"
)

// Implement our EthTest Manager
type TestManager struct {
	// stateManager *StateManager
	eventMux *InterfaceFeed

	db         common.Database
	txPool     *TxPool
	blockChain *BlockChain
	Blocks     []*types.Block
}

func (tm *TestManager) IsListening() bool {
	return false
}

func (tm *TestManager) IsMining() bool {
	return false
}

func (tm *TestManager) PeerCount() int {
	return 0
}

func (tm *TestManager) Peers() *list.List {
	return list.New()
}

func (tm *TestManager) BlockChain() *BlockChain {
	return tm.blockChain
}

func (tm *TestManager) TxPool() *TxPool {
	return tm.txPool
}

// func (tm *TestManager) StateManager() *StateManager {
// 	return tm.stateManager
// }

func (tm *TestManager) EventMux() *InterfaceFeed {
	return tm.eventMux
}

// func (tm *TestManager) KeyManager() *crypto.KeyManager {
// 	return nil
// }

func (tm *TestManager) Db() common.Database {
	return tm.db
}

func NewTestManager() *TestManager {
	db := ethdb.NewMemDatabase()

	testManager := &TestManager{}
	testManager.eventMux = new(InterfaceFeed)
	testManager.db = db
	// testManager.txPool = NewTxPool(testManager)
	// testManager.blockChain = NewBlockChain(testManager)
	// testManager.stateManager = NewStateManager(testManager)

	return testManager
}
