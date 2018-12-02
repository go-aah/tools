// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aahframe.work/console"
	"aahframe.work/essentials"
)

// Version no. of aah framework CLI tool
var Version = "0.13.0"

var (
	errVersionNotExists = errors.New("version not exists")
	verRegex            = regexp.MustCompile(`Version = "([\d.]+(\-edge|\-beta)?)"`)
)

// VersionPrinter method prints the versions info.
func VersionPrinter(c *console.Context) {
	cliLog = initCLILogger(nil)
	var err error
	aahVer, err = aahVersion(c)
	if err == nil && len(aahVer) > 0 {
		fmt.Printf("%-3s v%s\n", "aah", aahVer)
	}
	fmt.Printf("%-3s v%s\n", "cli", Version)
	if goVer := goVersion(); len(goVer) > 0 {
		fmt.Printf("%-3s v%s\n", "go", goVer)
	}
	if c.GlobalBool("buildinfo") {
		if len(CliCommitID) > 0 {
			fmt.Printf("commit sha %s (%s/%s)\n", CliCommitID, CliOS, CliArch)
		}
	}
}

func aahVersion(c *console.Context) (string, error) {
	// go.mod
	if ess.IsFileExists(aahProjectIdentifier) && ess.IsFileExists(goModIdentifier) && go111AndAbove {
		output, err := execCmd(gocmd, []string{"list", "-m", "-json", "all"}, false)
		if err != nil {
			cwd, _ := os.Getwd()
			if inferInsideGopath(cwd) && strings.Contains(err.Error(), "go list -m: not using modules") {
				logError("It seems aah project is using 'go.mod' and resides inside the GOPATH. Either move the aah project\n" +
					"outside the GOPATH or enable module support via setting 'GO111MODULE=on'.\n" +
					"For more info 'go help modules'.")
				exit(0)
			}
			return "", err
		}
		mods := parseGoListModJSON(output)
		for _, m := range mods {
			if m.Path == aahImportPath {
				return readVersionNo(m.Dir)
			}
		}
		return "", errors.New("aah import path not found")
	}

	// Vendor Directory
	importPath := appImportPath(c)
	if len(importPath) > 0 {
		vendorPath := filepath.Join(gosrcDir, importPath, "vendor")
		if ess.IsFileExists(vendorPath) {
			ver, _ := readVersionNo(filepath.Join(vendorPath, aahImportPath))
			if len(ver) > 0 && ver != "unknown" {
				return ver, nil
			}
		}
	}

	// GOPATH
	pkg, err := build.Import(aahImportPath, "", build.FindOnly)
	if err != nil {
		return "", nil
	}
	return readVersionNo(pkg.Dir)
}
