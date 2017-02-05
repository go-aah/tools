// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"os"
	"path/filepath"

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
)

var (
	runCmdFlags       = flag.NewFlagSet("run", flag.ExitOnError)
	runImportPathFlag = runCmdFlags.String("importPath", "", "Import path of aah application")
	runConfigFlag     = runCmdFlags.String("config", "", "External config for overriding aah.conf")
	runCmd            = &command{
		Name:      "run",
		UsageLine: "aah run [importPath] [config]",
		ArgsCount: 2,
		Short:     "run aah framework application",
		Long: `
Run the aah web/api application.

Example(s):

    aah run

    aah run -importPath=github.com/username/name

    aah run -importPath=github.com/username/name -config=/path/to/config/external.conf

Default aah application profile is 'dev'.`,
	}
)

func runRun(args []string) {
	if err := runCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	var (
		err         error
		importPath  string
		externalCfg *config.Config
	)

	if ess.IsStrEmpty(*runImportPathFlag) {
		importPath = importPathRelwd()
	} else {
		importPath = *runImportPathFlag
	}

	if !ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' does not exists", importPath)
	}

	var configPath string
	if !ess.IsStrEmpty(*runConfigFlag) {
		configPath, err = filepath.Abs(*runConfigFlag)
		if err != nil {
			log.Fatal(err)
		}

		externalCfg, err = config.LoadFile(configPath)
		if err != nil {
			log.Errorf("Unable to load external config: %s", err)
			log.Info("Move on with configuration from application")
		}
	}

	// REVIEW ...
	aah.Init(importPath)

	if externalCfg != nil {
		log.Infof("Applying config: %s", configPath)
		aah.MergeAppConfig(externalCfg)
	}

	// read build config from 'aah.project'
	aahProjectFile := filepath.Join(aah.AppBaseDir(), "aah.project")
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

	// TODO further implementation

}

func importPathRelwd() string {
	pwd, _ := os.Getwd()
	importPath, _ := filepath.Rel(gosrcDir, pwd)
	return filepath.ToSlash(importPath)
}

func init() {
	runCmd.Run = runRun
}
