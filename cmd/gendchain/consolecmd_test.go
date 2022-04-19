package main

import (
	"crypto/rand"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ChainAAS/gendchain/core"
	"github.com/ChainAAS/gendchain/params"
)

const (
	ipcAPIs  = "admin:1.0 clique:1.0 debug:1.0 eth:1.0 miner:1.0 net:1.0 personal:1.0 rpc:1.0 shh:1.0 txpool:1.0 web3:1.0"
	httpAPIs = "eth:1.0 net:1.0 rpc:1.0 web3:1.0"
)

// Tests that a node embedded within a console can be started up properly and
// then terminated by closing the input stream.
func TestConsoleWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"

	// Start an gendchain console, make sure it's cleaned up and terminate the console
	gendchain := runGendChain(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--etherbase", coinbase, "--shh",
		"console")

	// Gather all the infos the welcome message needs to contain
	gendchain.SetTemplateFunc("goos", func() string { return runtime.GOOS })
	gendchain.SetTemplateFunc("goarch", func() string { return runtime.GOARCH })
	gendchain.SetTemplateFunc("gover", runtime.Version)
	gendchain.SetTemplateFunc("gendchainver", func() string { return params.Version })
	gendchain.SetTemplateFunc("time", func() string {
		return time.Unix(int64(core.DefaultGenesisBlock().Timestamp), 0).Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
	})
	gendchain.SetTemplateFunc("apis", func() string { return ipcAPIs })

	// Verify the actual welcome message to the required template
	gendchain.Expect(`
Welcome to the GendChain JavaScript console!

instance: GendChain/v{{gendchainver}}/{{goos}}-{{goarch}}/{{gover}}
coinbase: {{.Etherbase}}
at block: 0 ({{time}})
 datadir: {{.Datadir}}
 modules: {{apis}}

> {{.InputLine "exit"}}
`)
	gendchain.ExpectExit()
}

// Tests that a console can be attached to a running node via various means.
func TestIPCAttachWelcome(t *testing.T) {
	// Configure the instance for IPC attachment
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	var ipc string
	if runtime.GOOS == "windows" {
		ipc = `\\.\pipe\geth` + strconv.Itoa(trulyRandInt(100000, 999999))
	} else {
		ws := tmpdir(t)
		defer os.RemoveAll(ws)
		ipc = filepath.Join(ws, "gendchain.ipc")
	}
	gendchain := runGendChain(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--etherbase", coinbase, "--shh", "--ipcpath", ipc)

	defer func() {
		gendchain.Interrupt()
		gendchain.ExpectExit()
	}()

	waitForEndpoint(t, ipc, 3*time.Second)
	testAttachWelcome(t, gendchain, "ipc:"+ipc, ipcAPIs)

}

func TestHTTPAttachWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	port := strconv.Itoa(trulyRandInt(1024, 65536)) // Yeah, sometimes this will fail, sorry :P
	gendchain := runGendChain(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--etherbase", coinbase, "--rpc", "--rpcport", port)
	defer func() {
		gendchain.Interrupt()
		gendchain.ExpectExit()
	}()

	endpoint := "http://127.0.0.1:" + port
	waitForEndpoint(t, endpoint, 3*time.Second)
	testAttachWelcome(t, gendchain, endpoint, httpAPIs)
}

func TestWSAttachWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	port := strconv.Itoa(trulyRandInt(1024, 65536)) // Yeah, sometimes this will fail, sorry :P

	gendchain := runGendChain(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--etherbase", coinbase, "--ws", "--wsport", port)
	defer func() {
		gendchain.Interrupt()
		gendchain.ExpectExit()
	}()

	endpoint := "ws://127.0.0.1:" + port
	waitForEndpoint(t, endpoint, 3*time.Second)
	testAttachWelcome(t, gendchain, endpoint, httpAPIs)
}

func testAttachWelcome(t *testing.T, gendchain *testGendchain, endpoint, apis string) {
	// Attach to a running gendchain note and terminate immediately
	attach := runGendChain(t, "attach", endpoint)
	defer attach.ExpectExit()
	attach.CloseStdin()

	// Gather all the infos the welcome message needs to contain
	attach.SetTemplateFunc("goos", func() string { return runtime.GOOS })
	attach.SetTemplateFunc("goarch", func() string { return runtime.GOARCH })
	attach.SetTemplateFunc("gover", runtime.Version)
	attach.SetTemplateFunc("gendchainver", func() string { return params.Version })
	attach.SetTemplateFunc("etherbase", func() string { return gendchain.Etherbase })
	attach.SetTemplateFunc("time", func() string {
		return time.Unix(int64(core.DefaultGenesisBlock().Timestamp), 0).Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
	})
	attach.SetTemplateFunc("ipc", func() bool { return strings.HasPrefix(endpoint, "ipc") })
	attach.SetTemplateFunc("datadir", func() string { return gendchain.Datadir })
	attach.SetTemplateFunc("apis", func() string { return apis })

	// Verify the actual welcome message to the required template
	attach.Expect(`
Welcome to the GendChain JavaScript console!

instance: GendChain/v{{gendchainver}}/{{goos}}-{{goarch}}/{{gover}}
coinbase: {{etherbase}}
at block: 0 ({{time}}){{if ipc}}
 datadir: {{datadir}}{{end}}
 modules: {{apis}}

> {{.InputLine "exit" }}
`)
	attach.ExpectExit()
}

// trulyRandInt generates a crypto random integer used by the console tests to
// not clash network ports with other tests running concurrently.
func trulyRandInt(lo, hi int) int {
	num, _ := rand.Int(rand.Reader, big.NewInt(int64(hi-lo)))
	return int(num.Int64()) + lo
}
