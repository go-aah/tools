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
	"path"
	"path/filepath"
	"strings"

	"aahframework.org/essentials"
	"aahframework.org/log"
)

const (
	modeWeb    = "web"
	modeAPI    = "api"
	aahTmplExt = ".atmpl"
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

	log.Infof("Your aah %s application '%s' created successfully at '%s'", mode, appName, appDir)
	log.Infof("You shall run your application: 'aah run -p=%s'\n", importPath)
}

func createAahApp(appDir, importPath, mode, appName string) error {
	aahToolsPath, err := build.Import("aahframework.org/tools/aah", "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}

	appTemplatePath := filepath.Join(aahToolsPath.Dir, "app-template")
	data := map[string]interface{}{
		"AppName":       appName,
		"AppMode":       mode,
		"AppImportPath": importPath,
		"TmplDemils":    "{{.}}",
	}

	// app directory creation
	if err := ess.MkDirAll(appDir, permRWXRXRX); err != nil {
		log.Fatal(err)
	}

	// aah.project
	processFile(appDir, appTemplatePath, filepath.Join(appTemplatePath, "aah.project.atmpl"), data)

	// gitignore
	processFile(appDir, appTemplatePath, filepath.Join(appTemplatePath, ".gitignore"), data)

	// source
	processSection(appDir, appTemplatePath, "app", data)

	// config
	processSection(appDir, appTemplatePath, "config", data)

	// i18n
	processSection(appDir, appTemplatePath, "i18n", data)

	if mode == modeWeb {
		// static
		processSection(appDir, appTemplatePath, "static", data)

		// views
		processSection(appDir, appTemplatePath, "views", data)
	}

	return nil
}

func processSection(destDir, srcDir, dir string, data map[string]interface{}) {
	files, _ := ess.FilesPath(filepath.Join(srcDir, dir))
	for _, v := range files {
		processFile(destDir, srcDir, v, data)
	}
}

func processFile(destDir, srcDir, f string, data map[string]interface{}) {
	dfPath := getDestPath(destDir, srcDir, f)
	dfDir := path.Dir(dfPath)
	if !ess.IsFileExists(dfDir) {
		_ = ess.MkDirAll(dfDir, permRWXRXRX)
	}

	sf, _ := os.Open(f)
	df, _ := os.Create(dfPath)

	if strings.HasSuffix(f, aahTmplExt) {
		sfbytes, _ := ioutil.ReadAll(sf)
		renderTmpl(df, string(sfbytes), data)
	} else {
		_, _ = io.Copy(df, sf)
	}

	_ = ess.ApplyFileMode(dfPath, permRWRWRW)
	ess.CloseQuietly(sf, df)
}

func getDestPath(destDir, srcDir, v string) string {
	dpath := v[len(srcDir):]
	dpath = filepath.Join(destDir, dpath)
	if strings.HasSuffix(v, aahTmplExt) {
		dpath = dpath[:len(dpath)-len(aahTmplExt)]
	}
	return dpath
}

func init() {
	newCmd.Run = newRun
}
