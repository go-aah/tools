// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

var (
	runCmdFlags            = flag.NewFlagSet("run", flag.ExitOnError)
	runImportPathFlag      = runCmdFlags.String("importPath", "", "Import path of aah application")
	runImportPathShortFlag = runCmdFlags.String("ip", "", "Import path of aah application")
	runConfigFlag          = runCmdFlags.String("config", "", "External config for overriding aah.conf")
	runConfigShortFlag     = runCmdFlags.String("c", "", "External config for overriding aah.conf")
	runProfileFlag         = runCmdFlags.String("profile", "", "Environment profile name to activate. e.g: dev, qa, prod")
	runProfileShortFlag    = runCmdFlags.String("p", "", "Environment profile name to activate. e.g: dev, qa, prod")
	runCmd                 = &command{
		Name:      "run",
		UsageLine: "aah run [-ip | -importPath] [-c | -config] [-p | -profile]",
		ArgsCount: 3,
		Short:     "run aah framework application",
		Long: `
Run the aah framework web/api application.

Example(s) short and long flag:
    aah run
		aah run -p=qa

		aah run -ip=github.com/user/appname
		aah run -ip=github.com/user/appname -p=qa
		aah run -ip=github.com/user/appname -c=/path/to/config/external.conf -p=qa

    aah run -importPath=github.com/username/name
		aah run -importPath=github.com/username/name -profile=qa
		aah run -importPath=github.com/username/name -config=/path/to/config/external.conf -profile=qa

Default aah application environment profile is 'dev'.

Note: It is recommended to use build and deploy approach instead of
using 'aah run' for production use.
`,
	}
)

func runRun(args []string) {
	if err := runCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	importPath := firstNonEmpty(*runImportPathFlag, *runImportPathShortFlag)
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' does not exists", importPath)
	}

	appStartArgs := []string{}
	configPath := getNonEmptyAbsPath(*runConfigFlag, *runConfigShortFlag)
	if !ess.IsStrEmpty(configPath) {
		appStartArgs = append(appStartArgs, "-config", configPath)
	}

	envProfile := firstNonEmpty(*runProfileFlag, *runProfileShortFlag)
	if !ess.IsStrEmpty(envProfile) {
		appStartArgs = append(appStartArgs, "-profile", envProfile)
	}

	aah.Init(importPath)

	buildCfg, err := loadAahProjectFile(aah.AppBaseDir())
	if err != nil {
		log.Fatalf("aah project file error: %s", err)
	}

	logLevel := buildCfg.StringDefault("build.log_level", "info")
	log.SetLevel(toLogLevel(logLevel))

	appBinary, err := compileApp(buildCfg)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := execCmd(appBinary, appStartArgs, true); err != nil {
		log.Fatal(err)
	}
}

func init() {
	runCmd.Run = runRun
}
