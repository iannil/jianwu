package main

import (
	"errors"
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
		var ie *cli.InfoError
		if errors.As(err, &ie) {
			fmt.Fprintf(os.Stderr, "jianwu: %v\n", err)
			return ie.Code
		}
		fmt.Fprintf(os.Stderr, "jianwu: %v\n", err)
		return cli.ExitCodeGeneric
	}
	return cli.ExitCodeSuccess
}
