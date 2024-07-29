package main

import (
	"fmt"
	"os"

	"github.com/eriktate/dr-todo/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
