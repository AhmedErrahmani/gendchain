
package tests

import (
	"testing"
)

func TestBlockchain(t *testing.T) {
	t.Parallel()

	bt := new(testMatcher)
	// General state tests are 'exported' as blockchain tests, but we can run them natively.
	bt.skipLoad(`^GeneralStateTests/`)
	// Skip random failures due to selfish mining test.
	bt.skipLoad(`^bcForgedTest/bcForkUncle\.json`)
	bt.skipLoad(`^bcMultiChainTest/(ChainAtoChainB_blockorder|CallContractFromNotBestBlock)`)
	bt.skipLoad(`^bcTotalDifficultyTest/(lotsOfLeafs|lotsOfBranches|sideChainWithMoreTransactions)`)
	// Slow tests
	bt.slow(`^bcExploitTest/DelegateCallSpam.json`)
	bt.slow(`^bcExploitTest/ShanghaiLove.json`)
	bt.slow(`^bcExploitTest/SuicideIssue.json`)
	bt.slow(`^bcForkStressTest/`)
	bt.slow(`^bcGasPricerTest/RPC_API_Test.json`)
	bt.slow(`^bcWalletTest/`)

	// Still failing tests that we need to look into
	//bt.fails(`^bcStateTests/suicideThenCheckBalance.json/suicideThenCheckBalance_Constantinople`, "TODO: investigate")
	//bt.fails(`^bcStateTests/suicideStorageCheckVCreate2.json/suicideStorageCheckVCreate2_Constantinople`, "TODO: investigate")
	//bt.fails(`^bcStateTests/suicideStorageCheckVCreate.json/suicideStorageCheckVCreate_Constantinople`, "TODO: investigate")
	//bt.fails(`^bcStateTests/suicideStorageCheck.json/suicideStorageCheck_Constantinople`, "TODO: investigate")

	bt.walk(t, blockTestDir, func(t *testing.T, name string, test *BlockTest) {
		if err := bt.checkFailure(t, name, test.Run()); err != nil {
			t.Error(err)
		}
	})
}
