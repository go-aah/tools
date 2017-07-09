// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

var runCmd = cli.Command{
	Name:    "run",
	Aliases: []string{"r"},
	Usage:   "Run aah framework application",
	Description: `Run the aah framework web/api application.

	Example(s) short and long flag:
    aah run
		aah run -p=qa

		aah run -ip=github.com/user/appname
		aah run -ip=github.com/user/appname -p=qa
		aah run -ip=github.com/user/appname -c=/path/to/config/external.conf -p=qa

    aah run --importPath=github.com/username/name
		aah run --importPath=github.com/username/name --profile=qa
		aah run --importPath=github.com/username/name --config=/path/to/config/external.conf --profile=qa

	Note: It is recommended to use build and deploy approach instead of
	using 'aah run' for production use.`,
	Action: runAction,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ip, importPath",
			Usage: "Import path of aah application",
		},
		cli.StringFlag{
			Name:  "p, profile",
			Usage: "Environment profile name to activate. e.g: dev, qa, prod",
			Value: "dev",
		},
		cli.StringFlag{
			Name:  "c, config",
			Usage: "External config for overriding aah.conf values",
		},
	},
}

func runAction(c *cli.Context) error {
	importPath := firstNonEmpty(c.String("ip"), c.String("importPath"))
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		fatalf("Given import path '%s' does not exists", importPath)
	}

	appStartArgs := []string{}
	configPath := getNonEmptyAbsPath(c.String("c"), c.String("config"))
	if !ess.IsStrEmpty(configPath) {
		appStartArgs = append(appStartArgs, "-config", configPath)
	}

	envProfile := firstNonEmpty(c.String("p"), c.String("config"))
	if !ess.IsStrEmpty(envProfile) {
		appStartArgs = append(appStartArgs, "-profile", envProfile)
	}

	aah.Init(importPath)

	buildCfg, err := loadAahProjectFile(aah.AppBaseDir())
	if err != nil {
		fatalf("aah project file error: %s", err)
	}

	_ = log.SetLevel(buildCfg.StringDefault("build.log_level", "info"))

	appBinary, err := compileApp(buildCfg, false)
	if err != nil {
		fatal(err)
	}

	if _, err := execCmd(appBinary, appStartArgs, true); err != nil {
		fatal(err)
	}

	return nil
}
