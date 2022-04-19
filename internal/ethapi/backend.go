
// Package ethapi implements the general Ethereum API functions.
package ethapi

import (
	"context"
	"math/big"

	"github.com/ChainAAS/gendchain/accounts"
	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/core"
	"github.com/ChainAAS/gendchain/core/state"
	"github.com/ChainAAS/gendchain/core/types"
	"github.com/ChainAAS/gendchain/core/vm"
	"github.com/ChainAAS/gendchain/eth/downloader"
	"github.com/ChainAAS/gendchain/params"
	"github.com/ChainAAS/gendchain/rpc"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// General Ethereum API
	Downloader() *downloader.Downloader
	ProtocolVersion() int
	SuggestPrice(ctx context.Context) (*big.Int, error)
	ChainDb() common.Database
	AccountManager() *accounts.Manager

	// BlockChain API
	SetHead(number uint64)
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error)
	GetTd(blockHash common.Hash) *big.Int
	GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, error)
	SubscribeChainEvent(ch chan<- core.ChainEvent, name string)
	UnsubscribeChainEvent(ch chan<- core.ChainEvent)
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent, name string)
	UnsubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent)
	SubscribeChainSideEvent(ch chan<- core.ChainSideEvent, name string)
	UnsubscribeChainSideEvent(ch chan<- core.ChainSideEvent)

	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	GetPoolTransactions() types.Transactions
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	Stats() (pending int, queued int)
	TxPoolContent(context.Context) (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent, string)
	UnsubscribeNewTxsEvent(chan<- core.NewTxsEvent)

	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	// InitialSupply returns the initial total supply from the genesis allocation,
	// or nil if a custom genesis is not available.
	InitialSupply() *big.Int
	// GenesisAlloc returns the initial genesis allocation, or nil if a custom genesis is not available.
	GenesisAlloc() core.GenesisAlloc
}

func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := newAddrLocker()
	return []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
			Public:    true,
		}, {
			Namespace: "txpool",
			Version:   "1.0",
			Service:   NewPublicTxPoolAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(apiBackend),
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(apiBackend.AccountManager()),
			Public:    true,
		}, {
			Namespace: "personal",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
			Public:    false,
		},
	}
}
