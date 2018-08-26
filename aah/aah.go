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

	"aahframe.work/aah/aruntime"
	"aahframe.work/aah/config"
	"aahframe.work/aah/essentials"
	"aahframe.work/aah/log"
	"gopkg.in/urfave/cli.v1"
)

const (
	permRWXRXRX   = 0755
	permRWRWRW    = 0666
	importPrefix  = "aahframe.work/aah"
	aahImportPath = "aahframe.work/aah"
)

var (
	gopath   string
	gocmd    string
	gosrcDir string
	gitcmd   string
	aahVer   string

	// abstract it, so we can do unit test
	fatal  = log.Fatal
	fatalf = log.Fatalf
	exit   = os.Exit

	// cli logger
	cliLog *log.Logger

	// CliPackaged is identify cli from go get or binary dist
	CliPackaged string
)

var errStopHere = errors.New("stop here")

func checkPrerequisites() error {
	gocmdName := goCmdName()
	// check go is installed or not
	if !ess.LookExecutable(gocmdName) {
		return fmt.Errorf("Unable to find '%s' executable in PATH", gocmdName)
	}

	var err error

	// get GOPATH, refer https://godoc.org/aahframework.org/essentials.v0#GoPath
	if gopath, err = ess.GoPath(); err != nil {
		return err
	}

	// Go executable
	if gocmd, err = exec.LookPath(gocmdName); err != nil {
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

	app := cli.NewApp()
	app.Name = "aah"
	app.Usage = "framework CLI tool"
	app.Version = Version
	app.Author = "Jeevanandam M."
	app.Email = "jeeva@myjeeva.com"
	app.Copyright = "Copyright (c) Jeevanandam M. <jeeva@myjeeva.com>"
	app.EnableBashCompletion = true

	app.Before = printHeader
	app.Commands = []cli.Command{
		newCmd,
		runCmd,
		buildCmd,
		listCmd,
		cleanCmd,
		switchCmd,
		updateCmd,
		generateCmd,
		migrateCmd,
	}

	// Global flags
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "y, yes",
			Usage: `Automatic yes to prompts. Assume "yes" as answer to all prompts and run non-interactively.`,
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	_ = app.Run(os.Args)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

func printHeader(c *cli.Context) error {
	aahVer, _ = aahVersion(c)
	hdr := fmt.Sprintf("aah framework v%s", aahVer)
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
	cli.HelpFlag = cli.BoolFlag{
		Name:  "h, help",
		Usage: "Shows help",
	}

	cli.VersionFlag = cli.BoolFlag{
		Name:  "v, version",
		Usage: "Prints cli, aah, go and aah libraries version",
	}

	cli.VersionPrinter = VersionPrinter

	cli.AppHelpTemplate = `Usage:
  {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
{{if .Commands}}
Commands:
{{range .Commands}}{{if not .HideHelp}}  {{join .Names ", "}}{{ "\t   " }}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
Global Options:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
`

	cli.CommandHelpTemplate = `Name:
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
`
}
