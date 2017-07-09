// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/essentials.v0"
)

var cleanCmd = cli.Command{
	Name:        "clean",
	Aliases:     []string{"c"},
	Usage:       "Cleans the aah generated files and build directory",
	Description: ``,
	Action:      cleanAction,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ip, importPath",
			Usage: "Import path of aah application",
		},
	},
}

func cleanAction(c *cli.Context) error {
	importPath := firstNonEmpty(c.String("ip"), c.String("importPath"))
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		fatalf("Given import path '%s' does not exists", importPath)
	}

	aah.Init(importPath)
	appBaseDir := aah.AppBaseDir()

	ess.DeleteFiles(filepath.Join(appBaseDir, "app", "aah.go"),
		filepath.Join(appBaseDir, "build"))

	fmt.Println("Import Path:", importPath, "clean successful.")
	fmt.Println()

	return nil
}
