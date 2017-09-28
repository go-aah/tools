// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/urfave/cli.v1"
)

const (
	releaseBranchName = "master"
	edgeBranchName    = "v0-unstable"
)

var switchCmd = cli.Command{
	Name:    "switch",
	Aliases: []string{"s"},
	Usage:   "Switch between aah release and edge version (beta)",
	Description: `Provides an ability to switch between aah release and edge version.

	Examples of short and long flags:
		aah s
		aah switch

	To check which version is currently active:
		aah s -w
		aah switch --whoami

	Note:
		- Currently it works with only GOPATH. Gradually I will add vendorize support too.
		- Currently it is in beta, help with your feedback for improvements.
		- It always operates on latest version, specific version is not supported.`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "w, whoami",
			Usage: "To know which version is currently active",
		},
	},
	Action: switchAction,
}

func switchAction(c *cli.Context) error {
	branchName := gitBranchName(libDir("aah"))
	if !strings.Contains(edgeBranchName+" "+releaseBranchName, branchName) {
		fmt.Printf("Unable to find out the version information.\n\n")
		return nil
	}

	if c.Bool("w") || c.Bool("whoami") {
		if branchName == releaseBranchName {
			fmt.Printf("You're using aah 'release' version.\n\n")
		} else {
			fmt.Printf("You're using aah 'edge' version, your feedback is appreciated.\n\n")
		}
		return nil
	}

	var toBranch string
	var friendlyName string
	if branchName == releaseBranchName {
		toBranch = edgeBranchName
		friendlyName = "edge"
	} else {
		toBranch = releaseBranchName
		friendlyName = "release"
	}

	fmt.Printf("Switching aah version to '%s' ...\n\n", friendlyName)

	// Enables git redirects for:
	// 	git config --global http.https://aahframework.org.followRedirects true
	// 	git config --global http.https://gopkg.in.followRedirects true
	//
	// Know more: https://github.com/git/git/commit/50d3413740d1da599cdc0106e6e916741394cc98
	enableGitRedirects()

	// Switch between release and edge version
	for _, lib := range libNames {
		dir := libDir(lib)
		// Checkout the branch
		if err := gitCheckout(dir, toBranch); err != nil {
			fatalf("Error occurred which switching aah version: %s", err)
		}

		// Refresh the branch codebase
		if err := gitPull(dir); err != nil {
			fatalf("Unable to refresh library: %s.%s", lib, versionSeries)
		}
	}

	// Refresh dependencies in grace mode
	if err := goGet(path.Join(importPrefix, "aah.v0", "...")); err != nil {
		fatalf("Unable to refresh dependencies: %s", err)
	}

	// Install aah CLI for the switched version
	args := []string{"install", path.Join(importPrefix, "tools.v0", "aah")}
	if _, err := execCmd(gocmd, args, false); err != nil {
		fatalf("Unable to compile CLI tool: %s", err)
	}

	if toBranch == releaseBranchName {
		fmt.Printf("You have successfully switched to aah 'release' version.\n\n")
	} else {
		fmt.Printf("You have successfully switched to aah 'edge' version, your feedback is appreciated.\n\n")
	}

	return nil
}
