package main

import (
	"fmt"
	"os"

	"github.com/moeart/ntr/cli"
	"golang.org/x/term"
)

func main() {
	// 保存原始终端状态
	oldState, err := term.GetState(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	
	err = cli.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
