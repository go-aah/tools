package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/go-aah/log"
)

var (
	isWindows = (runtime.GOOS == "windows")

	commands Commands

	header = `––––––––––––––––––––––––––––––––––––––
   aah  -  https://aahframework.org
––––––––––––––––––––––––––––––––––––––
`

	usageTemplate = `usage: aah command [arguments]

The commands are:
{{range .}}
    {{.Name | printf "%-12s"}} {{.Short}}{{end}}

Use "aah help [command]" for more information.

`
)

func main() {
	flag.Parse()
	args := flag.Args()

	printHeader()
	noOfArgs := len(args)
	if noOfArgs == 0 {
		displayUsage(1, usageTemplate, commands)
	}

	if args[0] == "help" {
		if noOfArgs > 1 {
			cmd, err := commands.Find(args[1])
			if err != nil {
				commandNotFound(args[1])
			}
			cmd.Usage()
		}
		displayUsage(0, usageTemplate, commands)
	}

	// if any panic happens recover and abort nice :)
	// otherwise paniccccccc........
	defer func() {
		if err := recover(); err != nil {
			if er, ok := err.(error); ok {
				abort(er, "this is unexpected!!!")
			}
			panic(err)
		}
	}()

	// find the command
	cmdName := args[0]
	cmd, err := commands.Find(cmdName)
	if err != nil {
		commandNotFound(cmdName)
	}

	// running request command
	cmd.Run(args[1:])
	return
}

func abort(err error, msg string) {
	log.Errorf("%v: %v\n", err, msg)
	os.Exit(1)
}

func printHeader() {
	if !isWindows {
		header = fmt.Sprintf("\033[1;32m%v\033[0m\n", header)
	}
	fmt.Fprintf(os.Stdout, header)
}

func init() {
	_ = log.SetPattern("%level:-5 %message")

	// Adding list of commands
	// The order here is the order in which they are printed by 'aah help'.
	commands = Commands{
		cmdNew,
	}
}
