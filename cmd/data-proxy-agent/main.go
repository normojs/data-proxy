package main

import (
	"os"

	"github.com/QuantumNous/new-api/pkg/dpagent"
)

var version = dpagent.DefaultAgentVersion

func main() {
	os.Exit(dpagent.RunCLI(os.Args[1:], os.Stdout, os.Stderr, version))
}
