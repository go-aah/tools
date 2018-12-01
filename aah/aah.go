// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"aahframe.work/aruntime"
	"aahframe.work/config"
	"aahframe.work/console"
	"aahframe.work/essentials"
	"aahframe.work/log"
)

const (
	permRWXRXRX   = os.FileMode(0755)
	permRWRWRW    = os.FileMode(0666)
	aahImportPath = "aahframe.work"
)

var (
	go111AndAbove bool
	gopath        string
	gocmd         string
	gosrcDir      string
	gitcmd        string
	aahVer        string

	// abstract it, so we can do unit test
	exit = os.Exit

	// cli logger
	cliLog *log.Logger

	// CliCommitID is the build git commit sha
	CliCommitID string

	// CliPackaged is to identify cli from go get or binary dist
	CliPackaged string

	// CliOS target build os name
	CliOS string

	// CliArch target build arch name
	CliArch string
)

var errStopHere = errors.New("stop here")

func checkPrerequisites() error {
	gocmdName := goCmdName()
	// check go is installed or not
	if !ess.LookExecutable(gocmdName) {
		return fmt.Errorf("Unable to find '%s' executable in PATH", gocmdName)
	}

	var err error

	// Go executable
	if gocmd, err = exec.LookPath(gocmdName); err != nil {
		return err
	}

	go111AndAbove = inferGo111AndAbove()
	if !go111AndAbove {
		logFatal("aah framework requires >= go1.11, since aah v0.12.0 and cli v0.13.0 release.")
	}

	// get GOPATH, refer https://godoc.org/aahframework.org/essentials.v0#GoPath
	if gopath, err = ess.GoPath(); err != nil {
		return err
	}

	// git
	if gitcmd, err = exec.LookPath("git"); err != nil {
		return err
	}

	gosrcDir = filepath.Join(gopath, "src")

	return nil
}

// aah cli tool entry point
func main() {
	cliLog = initCLILogger(nil)
	// if panic happens, recover and abort nicely :)
	defer func() {
		if r := recover(); r != nil {
			strace := aruntime.NewStacktrace(r, config.NewEmpty())
			strace.Print(os.Stdout)
			exit(2)
		}
	}()

	err := checkPrerequisites()
	if err == errStopHere {
		return
	} else if err != nil {
		logFatal(err)
	}

	app := console.NewApp()
	app.Name = "aah"
	app.Usage = "framework CLI tool"
	app.Version = Version
	app.Author = "Jeevanandam M."
	app.Email = "jeeva@myjeeva.com"
	app.Copyright = "Copyright (c) Jeevanandam M. <jeeva@myjeeva.com>"
	app.EnableBashCompletion = true

	app.Before = printHeader
	app.Commands = []console.Command{
		newCmd,
		runCmd,
		runConsoleCmd,
		buildCmd,
		listCmd,
		cleanCmd,
		generateCmd,
		migrateCmd,
	}

	// Global flags
	app.Flags = []console.Flag{
		console.BoolFlag{
			Name:  "yes, y",
			Usage: `Automatic yes to prompts. Assume "yes" as answer to all prompts and run non-interactively`,
		},
		console.BoolFlag{
			Name:  "buildinfo, b",
			Usage: `Build info flag works with version flag to display git commit sha, os and arch`,
		},
	}

	sort.Sort(console.FlagsByName(app.Flags))
	_ = app.Run(os.Args)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

func printHeader(c *console.Context) error {
	aahVer, _ = aahVersion(c)
	if len(aahVer) > 0 {
		aahVer = " v" + aahVer
	}
	hdr := "aah framework" + aahVer + " (cli v" + Version + ")"
	improveRpt := "# Report improvements/bugs at https://aahframework.org/issues #"
	cnt := len(improveRpt)
	sp := ((cnt - len(hdr)) / 2) - 1

	fmt.Println(chr2str("-", cnt))
	fmt.Println(chr2str(" ", sp) + hdr)
	fmt.Println(chr2str("-", cnt))
	fmt.Printf(improveRpt + "\n\n")

	return nil
}

func chr2str(chr string, cnt int) string {
	var str string
	for idx := 0; idx < cnt; idx++ {
		str += chr
	}
	return str
}

func init() {
	console.VersionFlagDesc("Prints aah, cli, aah and go version")
	console.HelpFlagDesc("Shows aah cli help")
	console.VersionPrinter(VersionPrinter)

	console.AppHelpTemplate(`Usage:
  {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
{{if .Commands}}
Commands:
{{range .Commands}}{{if not .HideHelp}}  {{join .Names ", "}}{{ "\t   " }}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
Global Options:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
`)

	console.CommandHelpTemplate(`Name:
  {{.HelpName}} - {{.Usage}}

Usage:
  {{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{if .Category}}

Category:
  {{.Category}}{{end}}{{if .Description}}

Description:
  {{.Description}}{{end}}{{if .VisibleFlags}}

Options:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`)
}
