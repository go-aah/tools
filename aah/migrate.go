// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// aahframework.org/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"aahframework.org/aah.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"gopkg.in/urfave/cli.v1"
)

const aahGrammarIdentifier = "migrate.grammar"

var migrateCmd = cli.Command{
	Name:    "migrate",
	Aliases: []string{"m"},
	Usage:   "Migrates application codebase to current version of aah (currently beta)",
	Description: `Command migrate is to house migration related sub-commands of aah.
  Currently it supports Go source code migrate.

	To know more about available 'migrate' sub commands:
		aah h m
		aah help migrate

	To know more about individual sub-commands details:
		aah m h c
		aah migrate help code
`,
	Subcommands: []cli.Command{
		cli.Command{
			Name:    "code",
			Aliases: []string{"c"},
			Usage:   "Migrates Go source code by making it compatible with current version of aah",
			Description: `Command code is to fix/upgrade aah's breaking changes and deprecated elements
  in Go source file to the current version of aah.

  The goal 'Code' command is to keep aah users always up-to-date with latest of aah; to take
  advantage of aah features and its capabilities.

	Example of script command:
		aah m c -i github.com/user/appname
		aah migrate code --importpath github.com/user/appname
			`,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "i, importpath",
					Usage: "Import path of aah application",
				},
			},
			Action: migrateCodeAction,
		},
	},
}

// TODO use golang.org/x/tools/imports

func migrateCodeAction(c *cli.Context) error {
	importPath := appImportPath(c)
	if err := aah.Init(importPath); err != nil {
		logFatal(err)
	}

	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)
	grammarFile := filepath.Join(aahToolsPath().Dir, aahGrammarIdentifier)
	grammarCfg, err := config.LoadFile(grammarFile)
	if err != nil {
		logFatal(err)
	}

	cliLog.Info("Note:")
	cliLog.Info("-----")
	cliLog.Info("Command operates based on grammer file. If you identify a new grammar entry, \n" +
		"create an issue here https://aahframework.org/issues, to include in the grammar file.\n")

	cliLog.Infof("Loaded grammar file: %s", grammarFile)
	cliLog.Infof("Loaded aah project file: %s", filepath.Join(aah.AppBaseDir(), aahProjectIdentifier))
	cliLog.Infof("Migrate starts for '%s' [%s]", aah.AppName(), aah.AppImportPath())

	// Go Source files
	if migrateGoSrcFiles(projectCfg, grammarCfg) == 0 {
		cliLog.Info("It seems application codebase 'app/**' is up-to-date")
	}

	cliLog.Infof("Migrate successful for '%s' [%s]\n", aah.AppName(), aah.AppImportPath())
	return nil
}

func migrateGoSrcFiles(projectCfg, grammarCfg *config.Config) int {
	grammar, found := grammarCfg.StringList("file.go.upgrades_replacer")
	if !found {
		logFatalf("Config 'file.go.upgrades_replacer' not found in grammar file")
	}
	fixer := strings.NewReplacer(grammar...)
	excludes, _ := projectCfg.StringList("build.ast_excludes")
	files, _ := ess.FilesPathExcludes(filepath.Join(aah.AppBaseDir(), "app"), true, ess.Excludes(excludes))
	count := 0
	for _, f := range files {
		df := strings.TrimPrefix(filepath.ToSlash(stripGoSrcPath(f)), aah.AppImportPath()+"/")
		fileBytes, err := ioutil.ReadFile(f)
		if err != nil {
			logError(err)
			cliLog.Infof("  |-- skipped: %s", df)
			continue
		}

		modFileBytes := []byte(fixer.Replace(string(fileBytes)))
		if bytes.Equal(fileBytes, modFileBytes) {
			// not modified
			continue
		}

		// file modified
		fmtFileBytes, err := format.Source(modFileBytes)
		if err != nil {
			logErrorf("While formating: %s", err)
			cliLog.Infof("  |-- skipped: %s", df)
			continue
		}

		if err = os.Truncate(f, 0); err != nil {
			logErrorf("While truncate: %s", err)
			cliLog.Infof("  |-- skipped: %s", df)
			continue
		}

		if err = ioutil.WriteFile(f, []byte(fmtFileBytes), permRWRWRW); err != nil {
			logError(err)
		}

		cliLog.Infof("  |-- processed: %s", df)
		count++
	}

	return count
}
