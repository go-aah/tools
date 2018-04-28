// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aruntime.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

const (
	permRWXRXRX  = 0755
	permRWRWRW   = 0666
	importPrefix = "aahframework.org"
)

var (
	gopath   string
	gocmd    string
	gosrcDir string
	aahVer   string

	// abstract it, so we can do unit test
	fatal  = log.Fatal
	fatalf = log.Fatalf
	exit   = os.Exit

	// cli logger
	cliLog *log.Logger
)

func checkPrerequisites() error {
	// check go is installed or not
	if !ess.LookExecutable("go") {
		return errors.New("Unable to find Go executable in PATH")
	}

	var err error

	// get GOPATH, refer https://godoc.org/aahframework.org/essentials.v0#GoPath
	if gopath, err = ess.GoPath(); err != nil {
		return err
	}

	if gocmd, err = exec.LookPath("go"); err != nil {
		return err
	}

	gosrcDir = filepath.Join(gopath, "src")

	if aahVer, err = aahVersion(); err == errVersionNotExists {
		return errors.New("aah framework is not installed, its easy to install. Run 'go get aahframework.org/tools.v0/aah'")
	}

	return nil
}

// aah cli tool entry point
func main() {
	// if panic happens, recover and abort nicely :)
	defer func() {
		if r := recover(); r != nil {
			strace := aruntime.NewStacktrace(r, config.NewEmptyConfig())
			strace.Print(os.Stdout)
			exit(2)
		}
	}()

	if err := checkPrerequisites(); err != nil {
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
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	_ = app.Run(os.Args)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

func printHeader(c *cli.Context) error {
	hdrCont := fmt.Sprintf("aah framework v%s", aahVer)
	improveRpt := "# Report improvements/bugs at https://aahframework.org/issues #"
	cnt := len(improveRpt)
	sp := (cnt - len(hdrCont)) / 2

	fmt.Println(chrtostr("=", cnt))
	fmt.Println(chrtostr(" ", sp) + hdrCont)
	fmt.Println(chrtostr("=", cnt))
	fmt.Printf(improveRpt + "\n\n")

	return nil
}

func chrtostr(chr string, cnt int) string {
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
