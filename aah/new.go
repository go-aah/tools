// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"

	"aahframework.org/essentials"
	"aahframework.org/log"
)

const (
	profileWeb = "web"
	profileAPI = "api"
)

var cmdNew = &command{
	Name:      "new",
	UsageLine: "aah new importPath [profile]",
	ArgsCount: 2,
	Short:     "create new aah 'web' or 'api' application",
	Long: `
Command 'new' creates aah application with given profile for quickly start.

Go to https://aahframework.org/getting-started to learn more and customize your application.

Parameter(s):
importPath      mandatory   e.g: github.com/user/appname
profile         optional    web or api, defalut is web

Example(s):
    aah new github.com/user/appname

    aah new github.com/user/appname api
`,
}

func newRun(args []string) {
	importPath, profile := parseNewArgs(args)
	if ess.IsImportPathExists(importPath) {
		abort(fmt.Errorf("Given import path '%s' already exists", importPath))
	}

	appDir := filepath.Join(gosrcDir, filepath.FromSlash(importPath))
	appName := filepath.Base(appDir)

	_ = importPath
	_ = appName
	_ = profile

}

func parseNewArgs(args []string) (string, string) {
	if len(args) == 0 {
		log.Errorf("No import path given. The usage is given below. Please have a look.\n")
		cmdNew.Usage()
	}

	profile := profileWeb
	if len(args) == 2 {
		switch args[1] {
		case profileWeb, profileAPI:
			profile = args[1]
		default:
			abort(fmt.Errorf("Unsupported new aah application profile: '%v', "+
				"please try with profile 'web' or 'api'", args[1]))
		}
	}

	return args[0], profile
}

func init() {
	cmdNew.Run = newRun
}
