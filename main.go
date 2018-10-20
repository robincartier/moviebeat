package main

import (
	"os"

	"github.com/robincartier/moviebeat/cmd"

	_ "github.com/robincartier/moviebeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
