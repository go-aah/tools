// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
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
	pkgCmdFlags       = flag.NewFlagSet("new", flag.ExitOnError)
	pkgImportPathFlag = pkgCmdFlags.String("importPath", "", "To create aah application")
	pkgCmd            = &command{
		Name:      "package",
		UsageLine: "aah package [importPath]",
		Flags:     pkgCmdFlags,
		ArgsCount: 1,
		Short:     "package aah application for deployment",
		Long: `
Package the aah web/api application by importPath.

To know more https://aahframework.org/tools/aah.

Example(s):
    aah package
    aah package -importPath=github.com/user/appname
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

	if ess.IsStrEmpty(*pkgImportPathFlag) {
		importPath = importPathRelwd()
	} else {
		importPath = *pkgImportPathFlag
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

	if err = buildApp(buildCfg); err != nil {
		log.Fatal(err)
	}

	pkgBaseDir, err := copyFilesIntoWorkingDir(buildCfg, appBaseDir)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("pkgBaseDir:", pkgBaseDir)
	archiveName, err := createZipArchive(pkgBaseDir, appBaseDir)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("archiveName:", archiveName)

}

func copyFilesIntoWorkingDir(buildCfg *config.Config, appBaseDir string) (string, error) {
	appBinary := createAppBinaryName(buildCfg)
	appBinaryName := filepath.Base(appBinary)

	tmpDir, err := ioutil.TempDir("", appBinaryName)
	if err != nil {
		return "", fmt.Errorf("unable to get temp directory: %s", err)
	}

	pkgBaseDir := filepath.Join(tmpDir, appBinaryName)
	ess.DeleteFiles(pkgBaseDir)
	if err = ess.MkDirAll(pkgBaseDir, 0755); err != nil {
		return "", err
	}

	// Binary file
	binDir := filepath.Join(pkgBaseDir, "bin")
	_ = ess.MkDirAll(binDir, 0755)
	_, _ = ess.CopyFile(binDir, appBinary)

	excludes, _ := buildCfg.StringList("build.ast_excludes")

	// config
	err = ess.CopyDir(pkgBaseDir, filepath.Join(appBaseDir, "config"), ess.Excludes(excludes))
	if err != nil {
		return "", err
	}

	// i18n
	err = ess.CopyDir(pkgBaseDir, filepath.Join(appBaseDir, "i18n"), ess.Excludes(excludes))
	if err != nil {
		return "", err
	}

	// static
	err = ess.CopyDir(pkgBaseDir, filepath.Join(appBaseDir, "static"), ess.Excludes(excludes))

	return pkgBaseDir, err
}

func createZipArchive(pkgBaseDir, appBaseDir string) (string, error) {

	return "", nil
}

func init() {
	pkgCmd.Run = pkgRun
}
