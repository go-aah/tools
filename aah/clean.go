// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"strings"

	"aahframe.work/aah"
	"aahframe.work/aah/essentials"
	"gopkg.in/urfave/cli.v1"
)

var cleanCmd = cli.Command{
	Name:    "clean",
	Aliases: []string{"c"},
	Usage:   "Cleans the aah generated files and build directory",
	Description: `Cleans the aah generated files and build directory.

	Such as aah.go and <app-base-dir>/build directory.

	Examples of short and long flags:
		aah clean`,
	Action: cleanAction,
}

func cleanAction(c *cli.Context) error {
	importPath := appImportPath(c)
	chdirIfRequired(importPath)
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

func cleanupAutoGenFiles(appBaseDir string) {
	appMainGoFile := filepath.Join(appBaseDir, "app", "aah.go")
	appBuildDir := filepath.Join(appBaseDir, "build")
	cliLog.Debugf("Cleaning %s", appMainGoFile)
	cliLog.Debugf("Cleaning build directory %s", appBuildDir)
	ess.DeleteFiles(appMainGoFile, appBuildDir)
}

func cleanupAutoGenVFSFiles(appBaseDir string) {
	vfsFiles, _ := filepath.Glob(filepath.Join(appBaseDir, "app", "aah_*_vfs.go"))
	if len(vfsFiles) > 0 {
		cliLog.Debugf("Cleaning embed files %s", strings.Join(vfsFiles, "\n\t"))
		ess.DeleteFiles(vfsFiles...)
	}
}
