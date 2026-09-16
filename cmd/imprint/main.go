package main

import (
	"os"

	"github.com/SteamedBread2333/imprint/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
