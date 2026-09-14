package main

import (
	"context"
	"fmt"
	"os"

	imcp "github.com/SteamedBread2333/imprint/internal/mcp"
)

func main() {
	cfg, err := imcp.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "imprint-mcp:", err.Error())
		os.Exit(2)
	}
	if cfg.Help {
		fmt.Print(imcp.Usage())
		return
	}
	if cfg.Version {
		imcp.PrintVersion(os.Stdout)
		return
	}
	if err := imcp.Run(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "imprint-mcp:", err.Error())
		os.Exit(1)
	}
}
