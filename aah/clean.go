// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"

	"aahframe.work"
	"aahframe.work/essentials"
	"gopkg.in/urfave/cli.v1"
)

var cleanCmd = cli.Command{
	Name:    "clean",
	Aliases: []string{"c"},
	Usage:   "Cleans the aah generated files and build directory",
	Description: `Cleans the aah generated files and build directory.

	Such as aah.go, aah*_vfs.go and <app-base-dir>/build directory.

	Examples of short and long flags:
		aah c
		aah clean`,
	Action: cleanAction,
}

func cleanAction(c *cli.Context) error {
	if !isAahProject() {
		logFatalf("Please go to aah application base directory and run '%s'.", strings.Join(os.Args, " "))
	}

	importPath := appImportPath(c)
	if ess.IsStrEmpty(importPath) {
		logFatalf("Unable to infer import path, ensure you're in the application base directory")
	}
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
