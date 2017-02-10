// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"aahframework.org/essentials"
	"aahframework.org/log"
)

const (
	modeWeb = "web"
	modeAPI = "api"
)

var (
	newCmdFlags       = flag.NewFlagSet("new", flag.ExitOnError)
	newImportPathFlag = newCmdFlags.String("importPath", "", "To create aah application")
	newModeFlag       = newCmdFlags.String("mode", modeWeb, "web or api, defalut is web")
	newCmd            = &command{
		Name:      "new",
		UsageLine: "aah new -importPath [profile]",
		Flags:     newCmdFlags,
		ArgsCount: 2,
		Short:     "create new aah 'web' or 'api' application",
		Long: `
Creates new aah application with given mode for quick start.

Go to https://aahframework.org/getting-started to learn more and customize your application.

Example(s):
    aah new -importPath=github.com/user/appname

    aah new -importPath=github.com/user/appname -mode=api
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

	if !(*newModeFlag == modeWeb || *newModeFlag == modeAPI) {
		log.Fatalf("Unsupported new aah application mode: '%v', "+
			"please try with mode 'web' or 'api'", *newModeFlag)
	}

	importPath := *newImportPathFlag
	mode := *newModeFlag

	if ess.IsImportPathExists(importPath) {
		log.Fatalf("Given import path '%s' already exists", importPath)
	}

	appDir := filepath.Join(gosrcDir, filepath.FromSlash(importPath))
	appName := filepath.Base(appDir)

	if err := createAahApp(appDir, importPath, mode, appName); err != nil {
		log.Fatal(err)
	}

	log.Infof("Your aah application '%s' created successfully at '%s'", appName, appDir)
	log.Infof("You shall run your application: 'aah run -p=%s'\n", importPath)
}

func createAahApp(appDir, importPath, mode, appName string) error {
	aahToolsPath, err := build.Import("aahframework.org/tools/aah", "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}

	appTemplatePath := filepath.Join(aahToolsPath.Dir, "app-template")
	data := map[string]interface{}{
		"AppName":    appName,
		"AppMode":    mode,
		"TmplDemils": "{{.}}",
	}

	// app directory creation
	if err := ess.MkDirAll(appDir, 0755); err != nil {
		log.Fatal(err)
	}

	// config
	processSection(appDir, appTemplatePath, "config", data)

	return nil
}

func processSection(destDir, srcDir, dir string, data map[string]interface{}) {
	files, _ := ess.FilesPath(filepath.Join(srcDir, dir))
	_ = ess.MkDirAll(filepath.Join(destDir, dir), 0755)
	for _, v := range files {
		dfPath := filepath.Join(destDir, dir, filenameFromPath(v))
		sf, _ := os.Open(v)
		df, _ := os.Create(dfPath)

		if strings.HasSuffix(v, ".atmpl") {
			sfbytes, _ := ioutil.ReadAll(sf)
			renderTmpl(df, string(sfbytes), data)
		} else {
			_, _ = io.Copy(df, sf)
		}
		_ = ess.ApplyFileMode(dfPath, 0766)
		ess.CloseQuietly(sf, df)
	}
}

func filenameFromPath(path string) string {
	name := filepath.Base(path)
	return name[:len(name)-len(".atmpl")]
}

func init() {
	newCmd.Run = newRun
}
