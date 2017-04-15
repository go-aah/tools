// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"aahframework.org/aah.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

var (
	buildCmdFlags              = flag.NewFlagSet("build", flag.ExitOnError)
	buildImportPathFlag        = buildCmdFlags.String("importPath", "", "Import path of aah application")
	buildImportPathShortFlag   = buildCmdFlags.String("ip", "", "Import path of aah application")
	buildArtifactPathFlag      = buildCmdFlags.String("artifactPath", "", "Output location application build artifact. Default location is <app-base>/aah-build")
	buildArtifactPathShortFlag = buildCmdFlags.String("ap", "", "Output location application build artifact. Default location is <app-base>/aah-build")
	buildCmd                   = &command{
		Name:      "build",
		UsageLine: "aah build [-ip | -importPath] [-ap | -artifactPath]",
		Flags:     buildCmdFlags,
		ArgsCount: 1,
		Short:     "build aah application for deployment",
		Long: `
Build the aah web/api application by importPath.

To know more CLI tool - https://docs.aahframework.org/doc=aah_cli

Example(s) short and long flag:
    aah build

    aah build -ip=github.com/user/appname -ap=/Users/jeeva

    aah build -importPath=github.com/user/appname -artifactPath=/Users/jeeva
`,
	}
)

func buildRun(args []string) {
	if err := buildCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	var (
		err        error
		importPath string
	)

	importPath = firstNonEmpty(*buildImportPathFlag, *buildImportPathShortFlag)
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' does not exists", importPath)
	}

	aah.Init(importPath)
	appBaseDir := aah.AppBaseDir()

	buildCfg, err := loadAahProjectFile(appBaseDir)
	if err != nil {
		log.Fatalf("aah project file error: %s", err)
	}

	logLevel := buildCfg.StringDefault("build.log_level", "info")
	log.SetLevel(toLogLevel(logLevel))

	appName := buildCfg.StringDefault("name", aah.AppName())
	log.Infof("Build starts for '%s' [%s]", appName, aah.AppImportPath())

	if _, err = compileApp(buildCfg); err != nil {
		log.Fatal(err)
	}

	buildBaseDir, err := copyFilesToWorkingDir(buildCfg, appBaseDir)
	if err != nil {
		log.Fatal(err)
	}

	appName = filepath.Base(buildBaseDir)
	archiveName := appName + "_" + getAppVersion(appBaseDir, buildCfg) + ".zip"
	defaultOutDir := filepath.Join(appBaseDir, "aah-build")
	destArchiveDir := firstNonEmpty(*buildArtifactPathFlag, *buildArtifactPathShortFlag, defaultOutDir)

	if ess.IsStrEmpty(*buildArtifactPathFlag) && ess.IsStrEmpty(*buildArtifactPathShortFlag) {
		_ = ess.DeleteFiles(defaultOutDir)
	}

	if err := createZipArchive(buildBaseDir, destArchiveDir, archiveName); err != nil {
		log.Fatal(err)
	}

	log.Infof("Build successful for '%s' [%s]", appName, aah.AppImportPath())
	log.Infof("Your application artifact is here: %s", filepath.Join(destArchiveDir, archiveName))
}

func copyFilesToWorkingDir(buildCfg *config.Config, appBaseDir string) (string, error) {
	appBinary := createAppBinaryName(buildCfg)
	appBinaryName := filepath.Base(appBinary)

	tmpDir, err := ioutil.TempDir("", appBinaryName)
	if err != nil {
		return "", fmt.Errorf("unable to get temp directory: %s", err)
	}

	buildBaseDir := filepath.Join(tmpDir, appBinaryName)
	ess.DeleteFiles(buildBaseDir)
	if err = ess.MkDirAll(buildBaseDir, permRWXRXRX); err != nil {
		return "", err
	}

	// binary file
	binDir := filepath.Join(buildBaseDir, "bin")
	_ = ess.MkDirAll(binDir, permRWXRXRX)
	_, _ = ess.CopyFile(binDir, appBinary)

	// apply executable file mode
	if err = ess.ApplyFileMode(filepath.Join(binDir, appBinaryName), permRWXRXRX); err != nil {
		log.Error(err)
	}

	// build package excludes
	cfgExcludes, _ := buildCfg.StringList("build.excludes")
	excludes := ess.Excludes(cfgExcludes)
	if err = excludes.Validate(); err != nil {
		log.Fatal(err)
	}

	// aah application and custom directories
	appDirs, _ := ess.DirsPath(appBaseDir, false)
	for _, srcdir := range appDirs {
		if excludes.Match(filepath.Base(srcdir)) {
			continue
		}

		if ess.IsFileExists(srcdir) {
			if err = ess.CopyDir(buildBaseDir, srcdir, excludes); err != nil {
				return "", err
			}
		}
	}

	// startup files
	data := map[string]string{"AppName": appBinaryName}
	buf := &bytes.Buffer{}
	renderTmpl(buf, aahBashStartupTemplate, data)
	if err = ioutil.WriteFile(filepath.Join(buildBaseDir, "aah"), buf.Bytes(), permRWXRXRX); err != nil {
		return "", err
	}

	buf.Reset()
	renderTmpl(buf, aahCmdStartupTemplate, data)
	err = ioutil.WriteFile(filepath.Join(buildBaseDir, "aah.cmd"), buf.Bytes(), permRWXRXRX)

	return buildBaseDir, err
}

func createZipArchive(buildBaseDir, archiveBaseDir, archiveName string) error {
	destZip := filepath.Join(archiveBaseDir, archiveName)
	_ = ess.DeleteFiles(destZip)
	if err := ess.MkDirAll(archiveBaseDir, permRWXRXRX); err != nil {
		log.Fatal(err)
	}
	return ess.Zip(destZip, buildBaseDir)
}

const aahBashStartupTemplate = `#!/usr/bin/env bash

###########################################
# aah application start up script for *NIX
###########################################

APP_NAME="{{.AppName}}"

# attempt to set APP_PATH
PRG="$0"
APP_PATH=$(cd "$(dirname $PRG)"; pwd)
APP_BIN_PATH="${APP_PATH}/bin"

# go to application path
cd "${APP_PATH}"

# start the application
exec "${APP_BIN_PATH}/${APP_NAME}"
`

const aahCmdStartupTemplate = `TITLE {{.AppName}}
@ECHO OFF

REM aah application start up script for Windows

SET APP_NAME={{.AppName}}

REM attempt to set APP_PATH
SET APP_PATH=%~dp0
SET APP_BIN_PATH=%APP_PATH%\bin

REM go to application path
cd %APP_PATH%

REM start the application
start %APP_BIN_PATH%\%APP_NAME%
`

func init() {
	buildCmd.Run = buildRun
}
