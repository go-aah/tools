// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"aahframework.org/essentials"
)

const (
	profileWeb = "web"
	profileAPI = "api"
)

var (
	newCmdFlags       = flag.NewFlagSet("new", flag.ExitOnError)
	newImportPathFlag = newCmdFlags.String("importPath", "", "To create aah application")
	newProfileFlag    = newCmdFlags.String("profile", profileWeb, "web or api, defalut is web")
	newCmd            = &command{
		Name:      "new",
		UsageLine: "aah new -importPath [profile]",
		Flags:     newCmdFlags,
		ArgsCount: 2,
		Short:     "create new aah 'web' or 'api' application",
		Long: `
Creates new aah application with given profile for quick start.

Go to https://aahframework.org/getting-started to learn more and customize your application.

Example(s):
    aah new -importPath=github.com/user/appname

    aah new -importPath=github.com/user/appname -profile=api
`,
	}
)

func newRun(args []string) {
	if err := newCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	if ess.IsStrEmpty(*newImportPathFlag) {
		newCmd.Usage()
	}

	if !(*newProfileFlag == profileWeb || *newProfileFlag == profileAPI) {
		log.Fatalf("Unsupported new aah application profile: '%v', "+
			"please try with profile 'web' or 'api'", *newProfileFlag)
	}

	importPath := *newImportPathFlag
	profile := *newProfileFlag

	if ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' already exists", importPath)
	}

	appDir := filepath.Join(gosrcDir, filepath.FromSlash(importPath))
	appName := filepath.Base(appDir)

	fmt.Println(importPath, profile, appName)

}

func init() {
	newCmd.Run = newRun
}
