package main

import (
	"fmt"
	"os"

	"github.com/zhurong/jianwu/internal/cli"
)

func main() {
	os.Exit(run())
}

func run() int {
	cmd := cli.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "jianwu: %v\n", err)
		return cli.ExitCodeGeneric
	}
	return cli.ExitCodeSuccess
}
