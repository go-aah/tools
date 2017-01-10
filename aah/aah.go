// Copyright (c) 2016 Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"aahframework.org/log"
)

var (
	// Version no. of aah CLI tool
	Version = "0.1"

	isWindows = (runtime.GOOS == "windows")

	commands Commands

	header = `––––––––––––––––––––––––––––––––––––––
   aah  -  https://aahframework.org
––––––––––––––––––––––––––––––––––––––
`

	usageTemplate = `Usage: aah command [arguments]

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
				abortm(er, "this is unexpected!!!")
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

	// Validate command arguments count
	if len(args)-1 > cmd.ArgsCount {
		log.Errorf("Too many arguments provided. The usage is given below. Please have a look.\n")
		cmd.Usage()
	}

	// running request command
	cmd.Run(args[1:])
	return
}

func abortm(err error, msg string) {
	log.Errorf("%v: %v\n", msg, err)
	os.Exit(1)
}

func abort(err error) {
	log.Errorf("%v\n", err)
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
