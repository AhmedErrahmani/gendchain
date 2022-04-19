// Package eth implements the GendChain protocol.
package eth

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ChainAAS/gendchain/accounts"
	"github.com/ChainAAS/gendchain/accounts/keystore"
	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/consensus"
	"github.com/ChainAAS/gendchain/consensus/clique"
	"github.com/ChainAAS/gendchain/core"
	"github.com/ChainAAS/gendchain/core/bloombits"
	"github.com/ChainAAS/gendchain/core/rawdb"
	"github.com/ChainAAS/gendchain/core/types"
	"github.com/ChainAAS/gendchain/core/vm"
	"github.com/ChainAAS/gendchain/eth/downloader"
	"github.com/ChainAAS/gendchain/eth/filters"
	"github.com/ChainAAS/gendchain/eth/gasprice"
	"github.com/ChainAAS/gendchain/internal/ethapi"
	"github.com/ChainAAS/gendchain/log"
	"github.com/ChainAAS/gendchain/miner"
	"github.com/ChainAAS/gendchain/node"
	"github.com/ChainAAS/gendchain/p2p"
	"github.com/ChainAAS/gendchain/params"
	"github.com/ChainAAS/gendchain/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// GendChain implements the GendChain full node service.
type GendChain struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan  chan bool    // Channel for shutting down the ethereum
	stopDbUpgrade func() error // stop chain db sequential key upgrade

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb common.Database // Block chain database

	eventMux       *core.InterfaceFeed
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend *EthApiBackend

	miner     *miner.Miner
	gasPrice  *big.Int // nil for default/dynamic
	etherbase common.Address

	networkId     uint64
	netRPCService *ethapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)
}

func (gc *GendChain) AddLesServer(ls LesServer) {
	gc.lesServer = ls
	ls.SetBloomBitsIndexer(gc.bloomIndexer)
}

// New creates a new GendChain object (including the
// initialisation of the common GendChain object)
func New(sctx *node.ServiceContext, config *Config) (*GendChain, error) {
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run eth.GendChain in light sync mode, use les.LightGendChain")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	chainDb, err := CreateDB(sctx, config, "chaindata")
	if err != nil {
		return nil, err
	}

	stopDbUpgrade := func() error { return nil } // upgradeDeduplicateData(chainDb)

	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.ConstantinopleOverride)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	if config.Genesis == nil {
		if genesisHash == params.MainnetGenesisHash {
			config.Genesis = core.DefaultGenesisBlock()
		}
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	if chainConfig.Clique == nil {
		return nil, fmt.Errorf("invalid configuration, clique is nil: %v", chainConfig)
	}
	eth := &GendChain{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       sctx.EventMux,
		accountManager: sctx.AccountManager,
		engine:         clique.New(chainConfig.Clique, chainDb),
		shutdownChan:   make(chan bool),
		stopDbUpgrade:  stopDbUpgrade,
		networkId:      config.NetworkId,
		gasPrice:       config.MinerGasPrice,
		etherbase:      config.Etherbase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	bcVersion := rawdb.ReadDatabaseVersion(chainDb.GlobalTable())
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}
	log.Info("Initialising GendChain protocol", "versions", ProtocolVersions, "network", config.NetworkId, "dbversion", dbVer)

	if !config.SkipBcVersionCheck {
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, GendChain %s only supports v%d", *bcVersion, params.Version, core.BlockChainVersion)
		} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
			log.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
			rawdb.WriteDatabaseVersion(chainDb.GlobalTable(), core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb.GlobalTable(), core.BlockChainVersion)
	}
	var (
		vmConfig    = vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, eth.chainConfig, eth.engine, vmConfig)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		if err := eth.blockchain.SetHead(compat.RewindTo); err != nil {
			log.Error("Cannot set head during chain rewind", "rewind_to", compat.RewindTo, "err", err)
		}
		rawdb.WriteChainConfig(chainDb.GlobalTable(), genesisHash, chainConfig)
	}
	eth.bloomIndexer.Start(eth.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = sctx.ResolvePath(config.TxPool.Journal)
	}
	eth.txPool = core.NewTxPool(config.TxPool, eth.chainConfig, eth.blockchain)

	if eth.protocolManager, err = NewProtocolManager(eth.chainConfig, config.SyncMode, config.NetworkId, eth.eventMux, eth.txPool, eth.engine, eth.blockchain, chainDb); err != nil {
		return nil, err
	}
	eth.miner = miner.New(eth, eth.chainConfig, eth.EventMux(), eth.engine, config.MinerRecommit, config.MinerGasFloor, config.MinerGasCeil, eth.isLocalBlock)
	if err := eth.miner.SetExtra(makeExtraData(config.MinerExtraData)); err != nil {
		log.Error("Cannot set extra chain data", "err", err)
	}

	eth.ApiBackend = &EthApiBackend{eth: eth}
	if g := eth.config.Genesis; g != nil {
		eth.ApiBackend.initialSupply = g.Alloc.Total()
	}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	eth.ApiBackend.gpo = gasprice.NewOracle(eth.ApiBackend, gpoParams)

	return eth, nil
}

// Example: 2.0.73/linux-amd64/go1.10.2
var defaultExtraData []byte
var defaultExtraDataOnce sync.Once

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		defaultExtraDataOnce.Do(func() {
			defaultExtraData = []byte(fmt.Sprintf("%s/%s-%s/%s", params.Version, runtime.GOOS, runtime.GOARCH, runtime.Version()))
			if uint64(len(defaultExtraData)) > params.MaximumExtraDataSize {
				defaultExtraData = defaultExtraData[:params.MaximumExtraDataSize]
			}
		})
		return defaultExtraData
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", string(extra), "limit", params.MaximumExtraDataSize)
		extra = extra[:params.MaximumExtraDataSize]
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (common.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (gc *GendChain) APIs() []rpc.API {
	apis := ethapi.GetAPIs(gc.ApiBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, gc.engine.APIs(gc.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(gc),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(gc),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(gc.protocolManager.downloader, gc.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(gc),
			Public:    false,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(gc.ApiBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(gc),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(gc),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(gc.chainConfig, gc),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   gc.netRPCService,
			Public:    true,
		},
	}...)
}

func (gc *GendChain) ResetWithGenesisBlock(gb *types.Block) {
	if err := gc.blockchain.ResetWithGenesisBlock(gb); err != nil {
		log.Error("Cannot reset with genesis block", "err", err)
	}
}

func (gc *GendChain) Etherbase() (eb common.Address, err error) {
	gc.lock.RLock()
	etherbase := gc.etherbase
	gc.lock.RUnlock()

	if etherbase != (common.Address{}) {
		return etherbase, nil
	}
	ks := gc.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	if wallets := ks.Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etherbase := accounts[0].Address

			gc.lock.Lock()
			gc.etherbase = etherbase
			gc.lock.Unlock()

			log.Info("Etherbase automatically configured", "address", etherbase)
			return etherbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

// isLocalBlock checks whether the specified block is mined
// by local miner accounts.
//
// We regard two types of accounts as local miner account: etherbase
// and accounts specified via `txpool.locals` flag.
func (gc *GendChain) isLocalBlock(block *types.Block) bool {
	author, err := gc.engine.Author(block.Header())
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", block.NumberU64(), "hash", block.Hash(), "err", err)
		return false
	}
	// Check whether the given address is etherbase.
	gc.lock.RLock()
	etherbase := gc.etherbase
	gc.lock.RUnlock()
	if author == etherbase {
		return true
	}
	// Check whether the given address is specified by `txpool.local`
	// CLI flag.
	for _, account := range gc.config.TxPool.Locals {
		if account == author {
			return true
		}
	}
	return false
}

// SetEtherbase sets the mining reward address.
func (gc *GendChain) SetEtherbase(etherbase common.Address) {
	gc.lock.Lock()
	gc.etherbase = etherbase
	gc.lock.Unlock()

	gc.miner.SetEtherbase(etherbase)
}

// StartMining starts the miner with the given number of CPU threads. If mining
// is already running, this method adjust the number of threads allowed to use
// and updates the minimum price required by the transaction pool.
func (gc *GendChain) StartMining(threads int) error {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := gc.engine.(threaded); ok {
		log.Info("Updated mining threads", "threads", threads)
		if threads == 0 {
			threads = -1 // Disable the miner from within
		}
		th.SetThreads(threads)
	}
	// If the miner was not running, initialize it
	if !gc.IsMining() {
		// Propagate the initial price point to the transaction pool
		gc.lock.RLock()
		price := gc.gasPrice
		gc.lock.RUnlock()
		gc.txPool.SetGasPrice(price)

		// Configure the local mining address
		eb, err := gc.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}
		if clique, ok := gc.engine.(*clique.Clique); ok {
			wallet, err := gc.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignData)
		}
		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		atomic.StoreUint32(&gc.protocolManager.acceptTxs, 1)

		go gc.miner.Start(eb)
	}
	return nil
}

// StopMining terminates the miner, both at the consensus engine level as well as
// at the block creation level.
func (gc *GendChain) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := gc.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	gc.miner.Stop()
}

func (gc *GendChain) IsMining() bool      { return gc.miner.Mining() }
func (gc *GendChain) Miner() *miner.Miner { return gc.miner }

func (gc *GendChain) AccountManager() *accounts.Manager  { return gc.accountManager }
func (gc *GendChain) BlockChain() *core.BlockChain       { return gc.blockchain }
func (gc *GendChain) TxPool() *core.TxPool               { return gc.txPool }
func (gc *GendChain) EventMux() *core.InterfaceFeed      { return gc.eventMux }
func (gc *GendChain) Engine() consensus.Engine           { return gc.engine }
func (gc *GendChain) ChainDb() common.Database           { return gc.chainDb }
func (gc *GendChain) IsListening() bool                  { return true } // Always listening
func (gc *GendChain) EthVersion() int                    { return int(gc.protocolManager.SubProtocols[0].Version) }
func (gc *GendChain) NetVersion() uint64                 { return gc.networkId }
func (gc *GendChain) Downloader() *downloader.Downloader { return gc.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (gc *GendChain) Protocols() []p2p.Protocol {
	if gc.lesServer == nil {
		return gc.protocolManager.SubProtocols
	}
	return append(gc.protocolManager.SubProtocols, gc.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// GendChain protocol implementation.
func (gc *GendChain) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	gc.startBloomHandlers()

	// Start the RPC service
	gc.netRPCService = ethapi.NewPublicNetAPI(srvr, gc.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if gc.config.LightServ > 0 {
		if gc.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", gc.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= gc.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	gc.protocolManager.Start(maxPeers)
	if gc.lesServer != nil {
		gc.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// GendChain protocol.
func (gc *GendChain) Stop() error {
	gc.bloomIndexer.Close()
	gc.blockchain.Stop()
	gc.protocolManager.Stop()
	if gc.lesServer != nil {
		gc.lesServer.Stop()
	}
	gc.txPool.Stop()
	gc.miner.Stop()
	gc.eventMux.Close()

	gc.chainDb.Close()
	close(gc.shutdownChan)

	return nil
}
