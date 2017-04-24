// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

const (
	typeWeb     = "web"
	typeAPI     = "api"
	storeCookie = "cookie"
	storeFile   = "file"
	aahTmplExt  = ".atmpl"
)

var (
	newCmd = &command{
		Name:      "new",
		UsageLine: "aah new",
		Short:     "create new aah 'web' or 'api' application (interactive)",
		Long: `
'aah new' command is an interactive program to assist you to quick start aah application.

Just provide your inputs based on your use case to generate base structure to kick
start your development.

Go to https://docs.aahframework.org to learn more and customize your aah application.
`,
	}
)

func newRun(args []string) {
	_ = log.SetPattern("%message")
	log.Info("\nWelcome to interactive way to create your aah application, press ^C to exit :)")
	log.Info()
	log.Info("Based on your inputs, aah CLI tool generates the aah application structure")
	log.Info("for you.")

	reader := bufio.NewReader(os.Stdin)

	// Collect data
	importPath := getImportPath(reader)
	appType := getAppType(reader)
	sessionScope, sessionStore := getSessionInfo(reader, appType)

	// Process it
	appDir := filepath.Join(gosrcDir, filepath.FromSlash(importPath))
	appName := filepath.Base(appDir)
	data := map[string]interface{}{
		"AppName":                 appName,
		"AppType":                 appType,
		"AppImportPath":           importPath,
		"AppSessionScope":         sessionScope,
		"AppSessionStore":         sessionStore,
		"AppSessionFileStorePath": filepath.Join(appDir, "sessions"),
		"AppSessionSignKey":       ess.RandomString(64),
		"AppSessionEncKey":        ess.RandomString(32),
		"TmplDemils":              "{{.}}",
	}

	if err := createAahApp(appDir, appType, data); err != nil {
		log.Fatal(err)
	}

	log.Infof("\nYour aah %s application created successfully at '%s'", appType, appDir)
	log.Infof("You shall run your application: 'aah run -importPath=%s'\n", importPath)
	log.Info("\nGo to https://docs.aahframework.org to learn more and customize your aah application.\n")
	_ = log.SetPattern(log.DefaultPattern)
}

func readInput(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Error(err)
		return ""
	}
	return strings.TrimSpace(input)
}

func getImportPath(reader *bufio.Reader) string {
	var importPath string
	for {
		importPath = readInput(reader, "\nEnter your application import path: ")
		if !ess.IsStrEmpty(importPath) {
			if ess.IsImportPathExists(importPath) {
				log.Errorf("Given import path '%s' is already exists", importPath)
				importPath = ""
				continue
			}
			break
		}
	}
	return importPath
}

func getAppType(reader *bufio.Reader) string {
	var appType string
	for {
		appType = readInput(reader, "\nChoose your application type (web or api), default is 'web': ")
		if ess.IsStrEmpty(appType) || appType == typeWeb || appType == typeAPI {
			break
		} else {
			log.Error("Unsupported new aah application type, choose either 'web or 'api")
			appType = ""
		}
	}
	if ess.IsStrEmpty(appType) {
		appType = typeWeb
	}
	return appType
}

func getSessionInfo(reader *bufio.Reader, appType string) (string, string) {
	sessionScope := "stateless"
	sessionStore := storeCookie
	if appType != typeWeb {
		return sessionScope, sessionStore
	}

	// Session Scope
	for {
		sessionManagement := readInput(reader, "\nDo you want 'stateful' HTTP session management, default is 'stateless' (Y/n): ")
		if sessionManagement == "Y" {
			sessionScope = "stateful"
			break
		} else if sessionManagement == "n" {
			sessionScope = "stateless"
			break
		}
	}

	if sessionScope == "stateful" {
		// Session Store
		for {
			sessionStore = readInput(reader, "\nChoose your session store (cookie or file), default is 'cookie': ")
			if ess.IsStrEmpty(sessionStore) || sessionStore == storeCookie || sessionStore == storeFile {
				break
			} else {
				log.Error("Unsupported session store type, choose either 'cookie or 'file")
				sessionStore = ""
			}
		}
		if ess.IsStrEmpty(sessionStore) {
			sessionStore = storeCookie
		}
	}

	return sessionScope, sessionStore
}

func createAahApp(appDir, appType string, data map[string]interface{}) error {
	aahToolsPath, err := build.Import(aahCLIImportPath, "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}

	appTemplatePath := filepath.Join(aahToolsPath.Dir, "app-template")

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

	if typeWeb == appType {
		// static
		processSection(appDir, appTemplatePath, "static", data)

		// views
		processSection(appDir, appTemplatePath, "views", data)
	}

	return nil
}

func processSection(destDir, srcDir, dir string, data map[string]interface{}) {
	files, _ := ess.FilesPath(filepath.Join(srcDir, dir), true)
	for _, v := range files {
		processFile(destDir, srcDir, v, data)
	}
}

func processFile(destDir, srcDir, f string, data map[string]interface{}) {
	dfPath := getDestPath(destDir, srcDir, f)
	dfDir := filepath.Dir(dfPath)
	if !ess.IsFileExists(dfDir) {
		_ = ess.MkDirAll(dfDir, permRWXRXRX)
	}

	sf, _ := os.Open(f)
	df, _ := os.Create(dfPath)

	if strings.HasSuffix(f, aahTmplExt) {
		sfbytes, _ := ioutil.ReadAll(sf)
		if err := renderTmpl(df, string(sfbytes), data); err != nil {
			log.Fatalf("Unable to process file '%s': %s", dfPath, err)
		}
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
