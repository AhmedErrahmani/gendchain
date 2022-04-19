package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ChainAAS/gendchain/internal/cmdtest"
	"github.com/ChainAAS/gendchain/rpc"
	"github.com/docker/docker/pkg/reexec"
)

func tmpdir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "gendchain-test")
	t.Logf("Temporary datadir %s created", dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

type testGendchain struct {
	*cmdtest.TestCmd

	// template variables for expect
	Datadir   string
	Etherbase string
}

func init() {
	// Run the app if we've been exec'd as "gendchain-test" in runGendChain.
	reexec.Register("gendchain-test", func() {
		if err := app.Run(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	})
}

func TestMain(m *testing.M) {
	// check if we have been reexec'd
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

// spawns gendchain with the given command line args. If the args don't set --datadir, the
// child g gets a temporary data directory.
func runGendChain(t *testing.T, args ...string) *testGendchain {
	tt := &testGendchain{}
	tt.TestCmd = cmdtest.NewTestCmd(t, tt)
	for i, arg := range args {
		switch {
		case arg == "-datadir" || arg == "--datadir":
			if i < len(args)-1 {
				tt.Datadir = args[i+1]
			}
		case arg == "-etherbase" || arg == "--etherbase":
			if i < len(args)-1 {
				tt.Etherbase = args[i+1]
			}
		}
	}
	if tt.Datadir == "" {
		tt.Datadir = tmpdir(t)
		tt.Cleanup = func() {
			os.RemoveAll(tt.Datadir)
		}
		args = append([]string{"-datadir", tt.Datadir}, args...)
		// Remove the temporary datadir if something fails below.
		defer func() {
			if t.Failed() {
				tt.Cleanup()
			}
		}()
	}

	// Boot "gendchain". This actually runs the test binary but the TestMain
	// function will prevent any tests from running.
	tt.Run("gendchain-test", args...)

	return tt
}

// waitForEndpoint attempts to connect to an RPC endpoint until it succeeds.
func waitForEndpoint(t *testing.T, endpoint string, timeout time.Duration) {
	probe := func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		c, err := rpc.DialContext(ctx, endpoint)
		if c != nil {
			_, err = c.SupportedModules()
			c.Close()
		}
		return err == nil
	}

	start := time.Now()
	for {
		if probe() {
			return
		}
		if time.Since(start) > timeout {
			t.Fatal("endpoint", endpoint, "did not open within", timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
