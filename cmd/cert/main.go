package main

import (
	"os"

	"github.com/fabedge/fabedge/pkg/cert"
)

func main() {
	command := cert.NewCertCommand()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
