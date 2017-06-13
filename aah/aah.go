// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"aahframework.org/aah.v0"
	"aahframework.org/aruntime.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

// Version no. of aah framework CLI tool
const Version = "0.7"

const (
	header = `–––––––––––––––––––––––––––––––––––––––––––––––––––––
   aah framework v%s -  https://aahframework.org
–––––––––––––––––––––––––––––––––––––––––––––––––––––
`
	aahImportPath    = "aahframework.org/aah.v0"
	aahCLIImportPath = "aahframework.org/tools.v0/aah"
	permRWXRXRX      = 0755
	permRWRWRW       = 0666
)

var (
	gopath   string
	gocmd    string
	gosrcDir string
	subCmds  commands

	// abstract it, so we can do unit test
	fatal  = log.Fatal
	fatalf = log.Fatalf
	exit   = os.Exit
)

// aah cli tool entry point
func main() {
	// if panic happens, recover and abort nicely :)
	defer func() {
		if r := recover(); r != nil {
			cfg, _ := config.ParseString(``)
			strace := aruntime.NewStacktrace(r, cfg)
			strace.Print(os.Stdout)
			exit(2)
		}
	}()

	// check go is installed or not
	if !ess.LookExecutable("go") {
		fatal("Unable to find Go executable in PATH")
	}

	var err error

	// get GOPATH, refer https://godoc.org/aahframework.org/essentials.v0#GoPath
	if gopath, err = ess.GoPath(); err != nil {
		fatal(err)
	}

	if gocmd, err = exec.LookPath("go"); err != nil {
		fatal(err)
	}

	flag.Parse()
	args := flag.Args()
	gosrcDir = filepath.Join(gopath, "src")

	printHeader()
	if len(args) == 0 {
		displayUsage()
	}

	// find the command
	cmd, err := subCmds.Find(args[0])
	if err != nil {
		commandNotFound(args[0])
	}

	// Validate command arguments count
	if len(args)-1 > cmd.ArgsCount {
		fatal("Too many arguments given. Run 'aah help command'.\n\n")
	}

	// running command
	cmd.Run(args[1:])
	return
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

func printHeader() {
	if isWindowsOS() {
		fmt.Fprintf(os.Stdout, header, aah.Version)
	} else {
		fmt.Fprintf(os.Stdout, fmt.Sprintf("\033[1;32m%v\033[0m", header), aah.Version)
	}
	fmt.Fprintf(os.Stdout, "# Report improvements/bugs at https://github.com/go-aah/aah/issues\n\n")
}

func init() {
	// Adding list of commands. The order here is the order in
	// which commands are printed by 'aah help'.
	subCmds = commands{
		newCmd,
		runCmd,
		buildCmd,
		listCmd,
		versionCmd,
		helpCmd,
	}
}
