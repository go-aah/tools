// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"path/filepath"

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
)

var (
	runCmdFlags            = flag.NewFlagSet("run", flag.ExitOnError)
	runImportPathFlag      = runCmdFlags.String("importPath", "", "Import path of aah application")
	runImportPathShortFlag = runCmdFlags.String("p", "", "Import path of aah application")
	runConfigFlag          = runCmdFlags.String("config", "", "External config for overriding aah.conf")
	runConfigShortFlag     = runCmdFlags.String("c", "", "External config for overriding aah.conf")
	runCmd                 = &command{
		Name:      "run",
		UsageLine: "aah run [-importPath | -p] [-config | -c]",
		ArgsCount: 2,
		Short:     "run aah framework application",
		Long: `
Run the aah web/api application.

Example(s):

    aah run

    aah run -importPath=github.com/username/name

		aah run -p=github.com/user/appname

    aah run -importPath=github.com/username/name -config=/path/to/config/external.conf

		aah run -p=github.com/user/appname -c=/path/to/config/external.conf

Default aah application profile is 'dev'.`,
	}
)

func runRun(args []string) {
	if err := runCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	var (
		err         error
		externalCfg *config.Config
	)

	importPath := firstNonEmpty(*runImportPathFlag, *runImportPathShortFlag)
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' does not exists", importPath)
	}

	configPath := getNonEmptyAbsPath(*runConfigFlag, *runConfigShortFlag)
	if !ess.IsStrEmpty(configPath) {
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

	appBinary, err := buildApp(buildCfg)
	if err != nil {
		log.Fatal(err)
	}

	_, err = execCmd(appBinary, []string{}, true)
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	runCmd.Run = runRun
}
