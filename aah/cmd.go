// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"aahframework.org/log.v0"
)

type (
	// Command structure insprired by `go` command and customized a bit
	// Reference: https://github.com/golang/go/blob/master/src/cmd/go/main.go
	command struct {
		// Run runs the command.
		// The args are the arguments after the command name.
		Run func(args []string)

		// Flags sub commands flag arguments
		Flags *flag.FlagSet

		// Name of the command
		Name string

		// UsageLine is the one-line usage message.
		UsageLine string

		// Total no of arguments (mandatory & optionals)
		ArgsCount int

		// Short is the short description shown in the 'aah help' output.
		Short string

		// Long is the long message shown in the 'aah help <this-command>' output.
		Long string
	}

	// Commands groups set of commands together and provides handy methods around it
	commands []*command
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Command methods
//___________________________________

// Usage displays the usage line and long description then exits
func (c *command) Usage() {
	fmt.Fprintf(os.Stderr, "Usage: %v\n\n", c.UsageLine)
	fmt.Fprintf(os.Stderr, "%v\n\n", strings.TrimSpace(c.Long))
	exit(2)
}

// Find finds the command from command name otherwise returns error
func (c *commands) Find(name string) (*command, error) {
	for _, cmd := range *c {
		if cmd.Name == name {
			return cmd, nil
		}
	}
	return nil, fmt.Errorf("command %v not found", name)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Helper methods for commands
//___________________________________

func displayUsage() {
	fmt.Fprintf(os.Stderr, "Usage: aah command [arguments]\n\n")
	fmt.Fprintf(os.Stderr, "Available commands:\n")
	for _, cmd := range subCmds {
		fmt.Fprintf(os.Stderr, "\t%-12s %s\n", cmd.Name, cmd.Short)
	}
	fmt.Fprintf(os.Stderr, "\nUse \"aah help [command]\" for more information about a command.\n\n")

	exit(2)
}

func commandNotFound(name string) {
	log.Errorf("Unknown command '%v', Run 'aah help'.\n\n", name)
	exit(2)
}
