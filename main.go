package main

import (
	"fmt"

	"github.com/moeart/ntr/cli"
)

func main() {
	err := cli.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
