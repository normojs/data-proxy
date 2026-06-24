package main

import (
	"os"
	"path/filepath"

	"github.com/QuantumNous/new-api/pkg/dpagent"
)

var version = dpagent.DefaultAgentVersion

func main() {
	os.Exit(dpagent.RunCLIWithProgram(filepath.Base(os.Args[0]), os.Args[1:], os.Stdout, os.Stderr, version))
}
