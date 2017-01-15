package main

import (
	"os"
	"path/filepath"

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/log"
)

var cmdRun = &command{
	Name:      "run",
	UsageLine: "run [importPath] [config]",
	ArgsCount: 2,
	Short:     "run aah framework application",
	Long: `
Run the aah web/api application.

Arguments:
importPath      optional    e.g: github.com/user/appname
config          optional    external config for override app.conf

Example(s):

    aah run

    aah run github.com/username/name

    aah run github.com/username/name /path/to/config/external.conf

Default aah application profile is 'dev'.`,
}

func runRun(args []string) {
	var (
		err         error
		importPath  string
		externalCfg *config.Config
	)

	if len(args) == 0 {
		importPath = importPathRelwd()
	} else {
		importPath = args[0]
	}

	if len(args) == 2 {
		var configPath string
		configPath, err = filepath.Abs(args[1])
		if err != nil {
			abort(err)
		}

		externalCfg, err = config.LoadFile(configPath)
		if err != nil {
			log.Errorf("Unable to load external config file[%s]: %s", args[1], err)
			log.Info("Move on with configuration from application")
		}
	}

	// IDEA ...
	// REVIEW ...
	aah.Init(importPath)

	if externalCfg != nil {
		aah.MergeAppConfig(externalCfg)
	}

	if err = buildApp(); err != nil {
		abort(err)
	}

	// TODO further implementation

}

func importPathRelwd() string {
	pwd, _ := os.Getwd()
	importPath, _ := filepath.Rel(gosrcDir, pwd)
	return filepath.ToSlash(importPath)
}

func init() {
	cmdRun.Run = runRun
}
