package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/ChainAAS/gendchain/cmd/evm/internal/compiler"

	cli "github.com/urfave/cli"
)

var compileCommand = cli.Command{
	Action:    compileCmd,
	Name:      "compile",
	Usage:     "compiles easm source to evm binary",
	ArgsUsage: "<file>",
}

func compileCmd(ctx *cli.Context) error {
	debug := ctx.GlobalBool(DebugFlag.Name)

	if len(ctx.Args().First()) == 0 {
		return errors.New("filename required")
	}

	fn := ctx.Args().First()
	src, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}

	bin, err := compiler.Compile(fn, src, debug)
	if err != nil {
		return err
	}
	fmt.Println(bin)
	return nil
}
