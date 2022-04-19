package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/ChainAAS/gendchain/internal/cmdtest"
	"github.com/docker/docker/pkg/reexec"
)

type testEthkey struct {
	*cmdtest.TestCmd
}

// spawns ethkey with the given command line args.
func runEthkey(t *testing.T, args ...string) *testEthkey {
	tt := new(testEthkey)
	tt.TestCmd = cmdtest.NewTestCmd(t, tt)
	tt.Run("ethkey-test", args...)
	return tt
}

func TestMain(m *testing.M) {
	// Run the app if we've been exec'd as "ethkey-test" in runEthkey.
	reexec.Register("ethkey-test", func() {
		if err := app.Run(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	})
	// check if we have been reexec'd
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}
