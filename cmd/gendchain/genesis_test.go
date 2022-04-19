package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type customGenesisTest struct {
	genesis string
	query   string
	result  string
}

var customGenesisTests = map[string]customGenesisTest{
	// Genesis file with an empty chain configuration (ensure missing fields work)
	"empty": {
		genesis: `{
			"alloc"      : {},
			"coinbase"   : "0x0000000000000000000000000000000000000000",
			"difficulty" : "0x20000",
			"extraData"  : "",
			"gasLimit"   : "0x2fefd8",
			"nonce"      : "0x0125864321546982",
			"mixhash"    : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"timestamp"  : "0x00",
			"config"     : {
				"clique": {
      				"period": 5,
      				"epoch": 3000
    			}
			}
		}`,
		query:  "eth.getBlock(0).nonce",
		result: "0x0125864321546982",
	},
	// Genesis file with specific chain configurations
	"specific": {
		genesis: `{
			"alloc"      : {},
			"coinbase"   : "0x0000000000000000000000000000000000000000",
			"difficulty" : "0x20000",
			"extraData"  : "",
			"gasLimit"   : "0x2fefd8",
			"nonce"      : "0x0000159876215648",
			"mixhash"    : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"timestamp"  : "0x00",
			"config"     : {
				"homesteadBlock" : 314,
				"clique": {
      				"period": 5,
      				"epoch": 3000
    			}
			}
		}`,
		query:  "eth.getBlock(0).nonce",
		result: "0x0000159876215648",
	},
}

// Tests that initializing GendChain with a custom genesis block and chain definitions
// work properly.
func TestCustomGenesis(t *testing.T) {
	for name, test := range customGenesisTests {
		t.Run(name, test.run)
	}
}
func (test customGenesisTest) run(t *testing.T) {
	// Create a temporary data directory to use and inspect later
	datadir := tmpdir(t)
	defer os.RemoveAll(datadir)

	// Initialize the data directory with the custom genesis block
	json := filepath.Join(datadir, "genesis.json")
	if err := ioutil.WriteFile(json, []byte(test.genesis), 0600); err != nil {
		t.Fatalf("failed to write genesis file: %v", err)
	}
	runGendChain(t, "--datadir", datadir, "init", json).WaitExit()

	// Query the custom genesis block
	gendchain := runGendChain(t,
		"--datadir", datadir, "--maxpeers", "0", "--port", "0",
		"--nodiscover", "--nat", "none", "--ipcdisable",
		"--exec", test.query, "console")
	gendchain.ExpectRegexp(test.result)
	gendchain.ExpectExit()
}
