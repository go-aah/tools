// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"aahframework.org"
	"gopkg.in/urfave/cli.v1"
)

var cleanCmd = cli.Command{
	Name:    "clean",
	Aliases: []string{"c"},
	Usage:   "Cleans the aah generated files and build directory",
	Description: `Cleans the aah generated files and build directory.

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
	importPath := appImportPath(c)

	if err := aah.Init(importPath); err != nil {
		logFatal(err)
	}
	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)

	cleanupAutoGenFiles(aah.AppBaseDir())
	cleanupAutoGenVFSFiles(aah.AppBaseDir())

	cliLog.Infof("Import Path '%v' clean successful.\n", importPath)

	return nil
}
