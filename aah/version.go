// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"go/build"
	"path/filepath"
	"regexp"
	"strings"

	"aahframe.work/aah/essentials"
	"gopkg.in/urfave/cli.v1"
)

// Version no. of aah framework CLI tool
const Version = "0.13.0-edge"

var (
	errVersionNotExists = errors.New("version not exists")
	verRegex            = regexp.MustCompile(`Version = "([\d.]+(\-edge)?)"`)
)

// VersionPrinter method prints the versions info.
func VersionPrinter(c *cli.Context) {
	cliLog = initCLILogger(nil)
	aahVer, _ = aahVersion(c)
	if len(aahVer) > 0 {
		fmt.Printf("%-3s v%s\n", "aah", aahVer)
	}
	fmt.Printf("%-3s v%s\n", "cli", Version)
	if goVer := goVersion(); len(goVer) > 0 {
		fmt.Printf("%-3s v%s\n", "go", goVer)
	}

	if c.Bool("all") { // currently not-executed intentionally
		fmt.Printf("\nLibraries:\n")
		for _, bd := range aahLibraryDirs() {
			bn := filepath.Base(bd)
			if strings.HasPrefix(bn, "aah") || strings.HasPrefix(bn, "tools") {
				continue
			}
			if verNo, err := readVersionNo(bd); err == nil {
				fmt.Printf("  %s v%s\n", bn[:len(bn)-3], verNo)
			}
		}
	}
	fmt.Println()
}

func aahVersion(c *cli.Context) (string, error) {
	// Vendor Directory
	importPath := importPathRelwd()
	if len(importPath) > 0 {
		vendorPath := filepath.Join(gosrcDir, importPath, "vendor")
		if ess.IsFileExists(vendorPath) {
			ver, _ := readVersionNo(filepath.Join(vendorPath, aahImportPath))
			if len(ver) > 0 && ver != "Unknown" {
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

func goVersion() string {
	if ver, err := execCmd(gocmd, []string{"version"}, false); err == nil {
		return strings.TrimLeft(strings.Fields(ver)[2], "go")
	}
	return ""
}
