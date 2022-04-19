
package core

import (
	"github.com/ChainAAS/gendchain/consensus"
	"github.com/ChainAAS/gendchain/core/state"
	"github.com/ChainAAS/gendchain/core/types"
	"github.com/ChainAAS/gendchain/core/vm"
	"github.com/ChainAAS/gendchain/crypto"
	"github.com/ChainAAS/gendchain/log"
	"github.com/ChainAAS/gendchain/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	txs := block.Transactions()
	header := block.Header()

	var (
		receipts = make(types.Receipts, len(txs))
		usedGas  = new(uint64)
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)

	// Create a new emv context and environment.
	evmContext := NewEVMContextLite(header, p.bc, nil)
	vmenv := vm.NewEVM(evmContext, statedb, p.config, cfg)
	signer := types.MakeSigner(p.config, header.Number)

	// Iterate over and process the individual transactions
	for i, tx := range txs {
		statedb.Prepare(tx.Hash(), block.Hash(), i)

		receipt, _, err := ApplyTransaction(vmenv, p.config, gp, statedb, header, tx, usedGas, signer)
		if err != nil {
			return nil, nil, 0, err
		}

		receipts[i] = receipt
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	_ = p.engine.Finalize(p.bc, header, statedb, block.Transactions(), receipts, false)
	log.Info("Processed Block", "number", header.Number, "hash", header.Hash(), "count", len(txs), "diff", header.Difficulty, "coinbase", header.Coinbase, "parent", header.ParentHash)

	return receipts, allLogs, *usedGas, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(vmenv *vm.EVM, config *params.ChainConfig, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, signer types.Signer) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage(signer)
	if err != nil {
		return nil, 0, err
	}

	vmenv.Context.Origin = msg.From()
	vmenv.Context.GasPrice = msg.GasPrice()
	vmenv.Reset()

	// Apply the transaction to the current state (included in the env)
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}
	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(header.Number) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = statedb.BlockHash()
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(statedb.TxIndex())

	return receipt, gas, err
}
