package main

import (
	"fmt"
	"os"

	"github.com/ChainAAS/gendchain/cmd/utils"
	"github.com/urfave/cli"
)

const (
	defaultKeyfileName = "keyfile.json"
)

// Git SHA1 commit hash of the release (set via linker flags)
var gitCommit = ""

var app *cli.App

func init() {
	app = utils.NewApp(gitCommit, "an Ethereum key manager")
	app.Commands = []cli.Command{
		commandGenerate,
		commandInspect,
		commandSignMessage,
		commandVerifyMessage,
	}
}

// Commonly used command line flags.
var (
	passphraseFlag = cli.StringFlag{
		Name:  "passwordfile",
		Usage: "the file that contains the password for the keyfile",
	}
	jsonFlag = cli.BoolFlag{
		Name:  "json",
		Usage: "output JSON instead of human-readable format",
	}
	messageFlag = cli.StringFlag{
		Name:  "message",
		Usage: "the file that contains the message to sign/verify",
	}
)

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
