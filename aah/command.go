package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

// Command structure insprired by `go` command
type Command struct {
	// Name of the command
	Name string

	// Run runs the command.
	// The args are the arguments after the command name.
	Run func(args []string)

	// UsageLine is the one-line usage message.
	UsageLine string

	// Short is the short description shown in the 'aah help' output.
	Short string

	// Long is the long message shown in the 'aah help <this-command>' output.
	Long string
}

// Usage displays the usage line and long description then exits
func (c *Command) Usage() {
	fmt.Fprintf(os.Stderr, "usage: %v\n\n", c.UsageLine)
	fmt.Fprintf(os.Stderr, "%v\n\n", strings.TrimSpace(c.Long))
	os.Exit(2)
}

// Commands groups set of commands together and provides handy methods around it
type Commands []*Command

// Find finds the command from command name otherwise returns error
func (c *Commands) Find(name string) (*Command, error) {
	for _, cmd := range *c {
		if cmd.Name == name {
			return cmd, nil
		}
	}
	return nil, fmt.Errorf("command %v not found", name)
}

// Helper methods for commands

func displayUsage(exitCode int, text string, data interface{}) {
	renderTmpl(os.Stdout, text, data)
	os.Exit(exitCode)
}

func commandNotFound(name string) {
	fmt.Printf("Command '%v' is not found, available commands and it's usage.\n\n", name)
	displayUsage(2, usageTemplate, commands)
}

func renderTmpl(w io.Writer, text string, data interface{}) {
	t := template.New("command")
	template.Must(t.Parse(text))
	if err := t.Execute(w, data); err != nil {
		panic(err)
	}
}
