// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aah.v0"
	"aahframework.org/essentials.v0"
)

var cleanCmd = cli.Command{
	Name:    "clean",
	Aliases: []string{"c"},
	Usage:   "Cleans the aah generated files and build directory",
	Description: `aah clean command does cleanup of generated files and build directory.

	Such as aah.go and <app-base-dir>/build directory.

	Examples of short and long flags:
		aah clean
		aah clean -i github.com/user/appname
		aah clean --importpath github.com/user/appname`,
	Action: cleanAction,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "i, importpath",
			Usage: "Import path of aah application",
		},
	},
}

func cleanAction(c *cli.Context) error {
	importPath := getAppImportPath(c)

	if err := aah.Init(importPath); err != nil {
		logFatal(err)
	}
	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)

	ess.DeleteFiles(
		filepath.Join(aah.AppBaseDir(), "app", "aah.go"),
		filepath.Join(aah.AppBaseDir(), "build"),
		filepath.Join(aah.AppBaseDir(), aah.AppName()+".pid"),
	)

	cliLog.Infof("Import Path '%v' clean successful.\n", importPath)

	return nil
}
