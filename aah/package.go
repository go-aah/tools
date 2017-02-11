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

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
)

var (
	pkgCmdFlags            = flag.NewFlagSet("new", flag.ExitOnError)
	pkgImportPathFlag      = pkgCmdFlags.String("importPath", "", "Import path of aah application")
	pkgImportPathShortFlag = pkgCmdFlags.String("p", "", "Import path of aah application")
	pkgCmd                 = &command{
		Name:      "package",
		UsageLine: "aah package [-importPath | -p]",
		Flags:     pkgCmdFlags,
		ArgsCount: 1,
		Short:     "package aah application for deployment",
		Long: `
Package the aah web/api application by importPath.

To know more https://aahframework.org/tools/aah.

Example(s):
    aah package

    aah package -importPath=github.com/user/appname

    aah package -p=github.com/user/appname
`,
	}
)

func pkgRun(args []string) {
	if err := pkgCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	var (
		err        error
		importPath string
	)

	importPath = firstNonEmpty(*pkgImportPathFlag, *pkgImportPathShortFlag)
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' does not exists", importPath)
	}

	aah.Init(importPath)
	appBaseDir := aah.AppBaseDir()

	// read build config from 'aah.project'
	aahProjectFile := filepath.Join(appBaseDir, "aah.project")
	if !ess.IsFileExists(aahProjectFile) {
		log.Fatal("Missing 'aah.project' file, not a valid aah application.")
	}

	log.Infof("Reading aah project file: %s", aahProjectFile)
	buildCfg, err := config.LoadFile(aahProjectFile)
	if err != nil {
		log.Fatalf("aah project file error: %s", err)
	}

	appName := buildCfg.StringDefault("name", aah.AppName())
	log.Infof("Starting package for '%s' [%s]", appName, aah.AppImportPath())

	if err = buildApp(buildCfg); err != nil {
		log.Fatal(err)
	}

	pkgBaseDir, err := copyFilesToWorkingDir(buildCfg, appBaseDir)
	if err != nil {
		log.Fatal(err)
	}

	appName = filepath.Base(pkgBaseDir)
	archiveName := appName + "_" + getAppVersion(appBaseDir, buildCfg) + ".zip"
	if err := createZipArchive(pkgBaseDir, appBaseDir, archiveName); err != nil {
		log.Fatal(err)
	}

	log.Infof("Package successful for '%s': %s", appName, archiveName)
}

func copyFilesToWorkingDir(buildCfg *config.Config, appBaseDir string) (string, error) {
	appBinary := createAppBinaryName(buildCfg)
	appBinaryName := filepath.Base(appBinary)

	tmpDir, err := ioutil.TempDir("", appBinaryName)
	if err != nil {
		return "", fmt.Errorf("unable to get temp directory: %s", err)
	}

	pkgBaseDir := filepath.Join(tmpDir, appBinaryName)
	ess.DeleteFiles(pkgBaseDir)
	if err = ess.MkDirAll(pkgBaseDir, permRWXRXRX); err != nil {
		return "", err
	}

	// Binary file
	binDir := filepath.Join(pkgBaseDir, "bin")
	_ = ess.MkDirAll(binDir, permRWXRXRX)
	_, _ = ess.CopyFile(binDir, appBinary)

	// apply executable file mode
	if err = ess.ApplyFileMode(filepath.Join(binDir, appBinaryName), permRWXRXRX); err != nil {
		log.Error(err)
	}

	cfgExcludes, _ := buildCfg.StringList("build.ast_excludes")
	excludes := ess.Excludes(cfgExcludes)

	// config
	if err = ess.CopyDir(pkgBaseDir, filepath.Join(appBaseDir, "config"), excludes); err != nil {
		return "", err
	}

	// i18n
	i18nPath := filepath.Join(appBaseDir, "i18n")
	if ess.IsFileExists(i18nPath) {
		if err = ess.CopyDir(pkgBaseDir, i18nPath, excludes); err != nil {
			return "", err
		}
	}

	// static
	staticPath := filepath.Join(appBaseDir, "static")
	if ess.IsFileExists(staticPath) {
		if err = ess.CopyDir(pkgBaseDir, staticPath, excludes); err != nil {
			return "", err
		}
	}

	// views
	viewsPath := filepath.Join(appBaseDir, "views")
	if ess.IsFileExists(viewsPath) {
		if err = ess.CopyDir(pkgBaseDir, viewsPath, excludes); err != nil {
			return "", err
		}
	}

	// startup files
	data := map[string]string{"AppName": appBinaryName}
	var buf bytes.Buffer
	renderTmpl(&buf, aahBashStartupTemplate, data)
	if err = ioutil.WriteFile(filepath.Join(pkgBaseDir, "aah"), buf.Bytes(), permRWXRXRX); err != nil {
		return "", err
	}

	buf.Reset()
	renderTmpl(&buf, aahCmdStartupTemplate, data)
	err = ioutil.WriteFile(filepath.Join(pkgBaseDir, "aah.cmd"), buf.Bytes(), permRWXRXRX)

	return pkgBaseDir, err
}

func createZipArchive(pkgBaseDir, appBaseDir, archiveName string) error {
	destZip := filepath.Join(appBaseDir, archiveName)
	_ = ess.DeleteFiles(destZip)

	return ess.Zip(destZip, pkgBaseDir)
}

const aahBashStartupTemplate = `#!/usr/bin/env bash

###########################################
# aah application start up script for UN*X
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
	pkgCmd.Run = pkgRun
}
