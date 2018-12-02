// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"

	"aahframe.work"
	"aahframe.work/console"
	"aahframe.work/essentials"
)

var runConsoleCmd = console.Command{
	Name:    "runcmd",
	Aliases: []string{"rc"},
	Usage:   "Runs aah application console command",
	Description: `Runs aah application console command. It's a handy development command
	to run user-defined console commands from aah CLI.

	Usage:
		aah runcmd <command-name-and-its-args>
	
	Example: Gonna run command 'vfs'
		aah runcmd vfs find --pattern "conf$"`,
	SkipFlagParsing: true,
	Action:          runConsoleCmdAction,
}

func runConsoleCmdAction(c *console.Context) error {
	if !isAahProject() {
		logFatalf("Please go to aah application base directory and run '%s'.", strings.Join(os.Args, " "))
	}
	importPath := appImportPath(c)
	if ess.IsStrEmpty(importPath) {
		logFatalf("Unable to infer import path, ensure you're in the aah application base directory")
	}
	chdirIfRequired(importPath)

	app := aah.App()
	if err := app.InitForCLI(importPath); err != nil {
		logFatal(err)
	}
	projectCfg := aahProjectCfg(app.BaseDir())
	cliLog = initCLILogger(projectCfg)
	checkAndGenerateInitgoFile(importPath, app.BaseDir(), app.Config())
	cliLog.Infof("Loaded aah project file: %s", filepath.Join(app.BaseDir(), aahProjectIdentifier))

	cleanupAutoGenFiles(app.BaseDir())

	appBinary, err := compileApp(&compileArgs{
		Cmd:        "RunConsoleCmd",
		ProjectCfg: projectCfg,
		AppPack:    false,
		AppEmbed:   false,
	})
	if err != nil {
		logFatal(err)
	}

	cliLog.Infof("Running application '%s' console command: '%s'",
		projectCfg.StringDefault("name", app.Name()), strings.Join(c.Args(), " "))
	if _, err := execCmd(appBinary, c.Args(), true); err != nil {
		logFatal(err)
	}
	return nil
}
