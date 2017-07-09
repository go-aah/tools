// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/ahttp.v0"
	"aahframework.org/aruntime.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/i18n.v0"
	"aahframework.org/log.v0"
	"aahframework.org/router.v0"
	"aahframework.org/security.v0"
	"aahframework.org/test.v0"
	"aahframework.org/view.v0"
)

// Version no. of aah framework CLI tool
const Version = "0.7"

const (
	header = `–––––––––––––––––––––––––––––––––––––––––––––––––––––
   aah framework v%s -  https://aahframework.org
–––––––––––––––––––––––––––––––––––––––––––––––––––––
`
	aahImportPath    = "aahframework.org/aah.v0-unstable"
	aahCLIImportPath = "aahframework.org/tools.v0/aah"
	permRWXRXRX      = 0755
	permRWRWRW       = 0666
)

var (
	gopath   string
	gocmd    string
	gosrcDir string

	// abstract it, so we can do unit test
	fatal  = log.Fatal
	fatalf = log.Fatalf
	exit   = os.Exit
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

	return nil
}

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

	if err := checkPrerequisites(); err != nil {
		fatal(err)
	}

	app := cli.NewApp()
	app.Name = "aah"
	app.Usage = "framework CLI tool"
	app.Version = Version
	app.Author = "Jeevanandam M."
	app.Email = "jeeva@myjeeva.com"
	app.Copyright = "Copyright (c) Jeevanandam M. <jeeva@myjeeva.com>"

	app.Before = printHeader
	app.Commands = []cli.Command{
		newCmd,
		runCmd,
		buildCmd,
		listCmd,
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	_ = app.Run(os.Args)
	return
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

func printHeader(c *cli.Context) error {
	if isWindowsOS() {
		fmt.Fprintf(c.App.Writer, header, aah.Version)
	} else {
		fmt.Fprintf(c.App.Writer, fmt.Sprintf("\033[1;32m%v\033[0m", header), aah.Version)
	}
	fmt.Fprintf(c.App.Writer, "# Report improvements/bugs at https://github.com/go-aah/aah/issues\n\n")
	return nil
}

func init() {
	cli.HelpFlag = cli.BoolFlag{
		Name:  "h, help",
		Usage: "show help",
	}

	cli.VersionFlag = cli.BoolFlag{
		Name:  "v, version",
		Usage: "print aah framework version and go version",
	}

	cli.VersionPrinter = func(c *cli.Context) {
		_ = printHeader(c)
		fmt.Fprint(c.App.Writer, "\nVersion(s):\n")
		fmt.Fprintf(c.App.Writer, "\t%-17s v%s\n", "aah framework", aah.Version)
		fmt.Fprintf(c.App.Writer, "\t%-17s v%s\n", "aah cli tool", Version)
		fmt.Fprintf(c.App.Writer, "\t%-17s %s\n", "Modules: ", strings.Join(
			[]string{
				"config v" + config.Version, "essentials v" + ess.Version,
				"ahttp v" + ahttp.Version, "router v" + router.Version,
				"security v" + security.Version}, ", "))
		fmt.Fprintf(c.App.Writer, "\t%-17s %s\n", "", strings.Join(
			[]string{"i18n v" + i18n.Version, "view v" + view.Version,
				"log v" + log.Version, "test v" + test.Version, "aruntime v" + aruntime.Version,
			}, ", "))
		fmt.Println()
		fmt.Fprintf(c.App.Writer, "\t%-17s %s\n", fmt.Sprintf("go[%s/%s]",
			runtime.GOOS, runtime.GOARCH), runtime.Version()[2:])
		fmt.Println()
	}

	cli.AppHelpTemplate = `USAGE:
  {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
{{if .Commands}}
COMMANDS:
{{range .Commands}}{{if not .HideHelp}}  {{join .Names ", "}}{{ "\t   " }}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}{{if .Version}}
VERSION:
  {{.Version}}
  {{end}}
`

	cli.CommandHelpTemplate = `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{if .Category}}

CATEGORY:
  {{.Category}}{{end}}{{if .Description}}

DESCRIPTION:
  {{.Description}}{{end}}{{if .VisibleFlags}}

OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`
}
