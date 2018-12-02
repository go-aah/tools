// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"aahframe.work"
	"aahframe.work/config"
	"aahframe.work/console"
	"aahframe.work/essentials"
)

const aahGrammarIdentifier = "migrate-0.12.x.conf"
const aahGrammarFetchLoc = "https://cdn.aahframework.org/" + aahGrammarIdentifier

var migrateCmd = console.Command{
	Name:    "migrate",
	Aliases: []string{"m"},
	Usage:   "Migrates application codebase to current version of aah (currently beta)",
	Description: `Command migrate is to house migration related sub-commands of aah.
  Currently it supports application Go source code and view files migration.

	To know more about available 'migrate' sub commands:
		aah help migrate

	To know more about individual sub-commands details:
		aah migrate help code`,
	Subcommands: []console.Command{
		{
			Name:    "code",
			Aliases: []string{"c"},
			Usage:   "Migrates application codebase by making it compatible with current version of aah",
			Description: `Command code is to fix/upgrade aah's breaking changes and deprecated elements
  in the application codebase to the current version of aah incrementally.

	Note: Migrate does not take file backup, assumes application use version control.

	Example:
		aah migrate code`,
			Action: migrateCodeAction,
		},
	},
}

func migrateCodeAction(c *console.Context) error {
	if !isAahProject() {
		logFatalf("Please go to aah application base directory and run '%s'.", strings.Join(os.Args, " "))
	}

	pwd, _ := os.Getwd()
	// createProjectInventory()
	_ = os.Chdir(pwd)

	grammarFile := filepath.Join(aahPath(), aahGrammarIdentifier)
	if ess.IsFileExists(grammarFile) {
		cliLog.Info("Refreshing migrate configuration from ", aahGrammarFetchLoc)
		fb, err := fetchURL(aahGrammarFetchLoc)
		if err == nil && fb.Len() > 0 {
			if err = ioutil.WriteFile(grammarFile, fb.Bytes(), permRWXRXRX); err == nil {
				cliLog.Info("Migrate configuration refreshed successfully")
			}
		} else {
			cliLog.Warnf("Unable to refresh migration configuration, error: %s", err)
			cliLog.Infof("Let's use the locally available config %s", grammarFile)
		}
	} else {
		cliLog.Info("Fetching migrate configuration from ", aahGrammarFetchLoc)
		fb, err := fetchURL(aahGrammarFetchLoc)
		if err != nil {
			logFatal(err)
		}
		if err = ioutil.WriteFile(grammarFile, fb.Bytes(), permRWXRXRX); err != nil {
			logFatal(err)
		}
	}
	grammarCfg, err := config.LoadFile(grammarFile)
	if err != nil {
		logFatal(err)
	}
	cliLog.Info("Loaded migrate configuration: ", grammarFile)

	importPath := appImportPath(c)
	app := aah.App()
	if err := app.InitForCLI(importPath); err != nil {
		logFatal(err)
	}
	appBaseDir := app.BaseDir()
	projectIsInGoPath := isInGoPath(appBaseDir)
	projectCfg := aahProjectCfg(appBaseDir)
	cliLog.Info("Loaded aah project file: ", filepath.Join(appBaseDir, aahProjectIdentifier))
	cliLog = initCLILogger(projectCfg)

	cliLog.Warn("Migrate command does not take file backup. Command assumes application use version control.")
	if c.GlobalBool("yes") {
		fmt.Println("Would you like to continue? [y/N]: y")
	} else if !collectYesOrNo(reader, "Would you like to continue? [y/N]") {
		cliLog.Info("Okay, I respect your choice. Bye.")
		return nil
	}

	cliLog.Info("\nNote:")
	cliLog.Info("-----")
	cliLog.Infof("Command works based on file '%s'.\n"+
		"If you identify a missing grammar entry, create an issue at https://aahframework.org/issues.\n",
		grammarFile)
	cliLog.Infof("Code migration starts for '%s' [%s]", app.Name(), app.ImportPath())

	// Go Source files
	cliLog.Infof("Go source code migration starts ...")
	if migrateGoSrcFiles(projectCfg, grammarCfg) == 0 {
		cliLog.Info("  |-- It seems application Go source code are up-to-date")
	} else {
		cliLog.Infof("Go source code migration successful")
	}

	// View files
	if ess.IsFileExists(filepath.Join(app.BaseDir(), "views")) {
		cliLog.Infof("View file migration starts ...")
		if migrateViewFiles(projectCfg, grammarCfg) == 0 {
			cliLog.Info("  |-- It seems application view files are up-to-date")
		} else {
			cliLog.Infof("View file migration successful")
		}
	}

	// .gitignore
	if ess.IsFileExists(".gitignore") {
		gitIgnore, err := ioutil.ReadFile(".gitignore")
		if err != nil {
			cliLog.Error("Unable to read .gitignore file, so no auto migration. Please do it manually.")
		} else {
			gitIgnoreStr := string(gitIgnore)
			replaceSet := []string{"build/"} // will be updated to "# aah application - end"
			for _, entry := range []string{
				"aah_*_vfs.go", "app/generated", "vendor/*/", "# aah application - end",
			} {
				if !strings.Contains(gitIgnoreStr, entry) {
					replaceSet = append(replaceSet, entry)
				}
			}
			if len(replaceSet) == 0 {
				cliLog.Info("It seems file '.gitignore' are up-to-date")
			} else {
				cliLog.Info("Updating file '.gitignore' ...")
				fixer := strings.NewReplacer("build/", strings.Join(replaceSet, "\n"))
				gitIgnore = []byte(fixer.Replace(gitIgnoreStr))
				if err = ioutil.WriteFile(".gitignore", gitIgnore, permRWXRXRX); err != nil {
					cliLog.Error(err)
				}
			}
		}
	}

	// go mod
	aahBasePath := filepath.ToSlash(aahPath())
	appTmplBaseDir := inferAppTmplBaseDir()
	if ess.IsStrEmpty(appTmplBaseDir) {
		logFatalf("Unable to find aah app template at %s/.aah/app-templates", aahBasePath)
	}
	cliLog.Infof("Creating file 'go.mod' for %s ...", app.Name())
	if ess.IsFileExists("go.mod") {
		cliLog.Info("File 'go.mod' already exists, so let's update it")
		aahLibImports, found := grammarCfg.StringList("file.go.official_modules")
		if found {
			goModBytes, err := ioutil.ReadFile("go.mod")
			if err != nil {
				logFatal(err)
			}
			for _, imp := range aahLibImports {
				// if go.mod file contains import path then update it
				if bytes.Contains(goModBytes, []byte(imp+" v")) {
					if _, err = execCmd(gocmd, []string{"get", imp + "@latest"}, false); err != nil {
						logError(err)
					}
				}
			}
		}
	} else {
		modImportPath := filepath.Base(app.ImportPath())
		if projectIsInGoPath {
			modImportPath = filepath.ToSlash(stripGoSrcPath(appBaseDir))
		} else {
			cliLog.Warn("Please check the file 'go.mod' and update your application import path.")
		}
		data := &appTmplData{ImportPath: modImportPath}
		if ess.IsFileExists("views") {
			data.Type = typeWeb
		}
		processFile(appBaseDir, file{
			src: filepath.Join(appTmplBaseDir, "go.mod.atmpl"),
			dst: filepath.Join(appBaseDir, "go.mod.atmpl"),
		}, map[string]interface{}{
			"App": data,
		})
	}

	cliLog.Infof("Code migration successful for '%s' [%s]\n", app.Name(), app.ImportPath())
	if projectIsInGoPath {
		cliLog.Warn("PLEASE MOVE YOUR aah PROJECT OUTSIDE THE 'GOPATH'.")
	}
	cliLog.Warn("REVIEW YOUR APPLICATION CHANGES, VERIFY AND COMMIT IT.")
	return nil
}

func migrateGoSrcFiles(projectCfg, grammarCfg *config.Config) int {
	count := 0
	levelKeys, found := grammarCfg.StringList("file.go.levels")
	if !found || len(levelKeys) == 0 {
		cliLog.Errorf("Grammar definitions not found in migration config file")
		return count
	}

	for _, keyName := range levelKeys {
		grammar, found := grammarCfg.StringList("file.go." + keyName)
		if !found {
			continue
		}
		cliLog.Infof("Processing %s", strings.Replace(keyName, "_", " ", -1))
		fixer := strings.NewReplacer(grammar...)
		excludes, _ := projectCfg.StringList("build.ast_excludes")
		files, _ := ess.FilesPathExcludes(filepath.Join(aah.App().BaseDir(), "app"), true, ess.Excludes(excludes))
		for _, f := range files {
			if filepath.Ext(f) != ".go" {
				continue
			}
			if !migrateFile(f, fixer) {
				continue
			}
			count++
		}
	}
	return count
}

func migrateViewFiles(projectCfg, grammarCfg *config.Config) int {
	count := 0
	levelKeys, found := grammarCfg.StringList("file.view.levels")
	if !found || len(levelKeys) == 0 {
		cliLog.Errorf("Grammar definitions not found in migration config file")
		return count
	}

	app := aah.App()
	fileExt := app.Config().StringDefault("view.ext", ".html")
	delimiters := strings.Split(app.Config().StringDefault("view.delimiters", "{{.}}"), ".")
	for _, keyName := range levelKeys {
		rules := grammarCfg.KeysByPath("file.view." + keyName)
		for _, rule := range rules {
			skipCheckStr := strings.TrimSpace(grammarCfg.StringDefault(
				fmt.Sprintf("file.view.%s.%s.skip_check", keyName, rule), ""))
			grammar, found := grammarCfg.StringList(
				fmt.Sprintf("file.view.%s.%s.grammar", keyName, rule))
			if !found {
				continue
			}
			for i := 0; i < len(grammar); i++ {
				grammar[i] = strings.Replace(strings.Replace(grammar[i], "%delim_start%", delimiters[0], -1), "%delim_end%", delimiters[1], -1)
			}
			cliLog.Infof("Processing %s", strings.Replace(keyName, "_", " ", -1))
			files, _ := ess.FilesPath(filepath.Join(app.BaseDir(), "views"), true)
			fixer := strings.NewReplacer(grammar...)
			for _, f := range files {
				if filepath.Ext(f) != fileExt {
					continue
				}
				if len(skipCheckStr) > 0 {
					b, err := ioutil.ReadFile(f)
					if err != nil {
						cliLog.Error(err)
						continue
					}
					if strings.Contains(string(b), skipCheckStr) {
						continue
					}
				}
				if !migrateFile(f, fixer) {
					continue
				}
				count++
			}
		}
	}
	return count
}

func migrateFile(f string, fixer *strings.Replacer) bool {
	df := filepath.ToSlash(strings.TrimPrefix(f, aah.App().BaseDir()+"/"))
	if isInGoPath(aah.App().BaseDir()) {
		df = strings.TrimPrefix(filepath.ToSlash(stripGoSrcPath(f)), aah.App().ImportPath()+"/")
	}
	fileBytes, err := ioutil.ReadFile(f)
	if err != nil {
		logError(err)
		cliLog.Infof("  |-- skipped: %s", df)
		return false
	}

	modFileBytes := []byte(fixer.Replace(string(fileBytes)))
	if bytes.Equal(fileBytes, modFileBytes) {
		// not modified
		return false
	}

	if filepath.Ext(f) == ".go" {
		// format go src file
		// var err error
		if modFileBytes, err = format.Source(modFileBytes); err != nil {
			logErrorf("While formating: %s", err)
			cliLog.Infof("  |-- skipped: %s", df)
			return false
		}
	}

	if err = os.Truncate(f, 0); err != nil {
		logErrorf("While truncate: %s", err)
		cliLog.Infof("  |-- skipped: %s", df)
		return false
	}

	if err = ioutil.WriteFile(f, modFileBytes, permRWRWRW); err != nil {
		logError(err)
		cliLog.Infof("  |-- [ERROR] processing: %s", df)
	} else {
		cliLog.Infof("  |-- processed: %s", df)
	}

	return true
}

func checkAndGenerateInitgoFile(importPath, baseDir string, appCfg *config.Config) {
	initGoFile := filepath.Join(baseDir, "app", "init.go")
	if !ess.IsFileExists(initGoFile) {
		cliLog.Warn("***** In aah v0.10 'init.go' file introduced to evolve aah framework." +
			" Since its not found, generating 'init.go' file. Please add 'init.go' into VCS. *****\n")

		appTmplBaseDir := inferAppTmplBaseDir()
		if ess.IsStrEmpty(appTmplBaseDir) {
			logFatalf("Unable to find aah app template at %s/.aah/app-templates", aahPath())
		}
		appType := typeAPI
		if ess.IsFileExists(filepath.Join(baseDir, "views")) {
			appType = typeWeb
		}
		data := map[string]interface{}{
			"App": &appTmplData{
				Type:       appType,
				ViewEngine: appCfg.StringDefault("view.engine", "go"),
			},
		}

		processFile(baseDir, file{
			src: filepath.Join(appTmplBaseDir, "app", "init.go.atmpl"),
			dst: filepath.Join(baseDir, "app", "init.go"),
		}, data)
	}
}
