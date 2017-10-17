// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

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

	To refresh currently active aah codebase version to the latest:
		aah s -r
		aah switch --refresh

	Note:
		- Currently it works with only GOPATH. Gradually I will add vendorize support too.
		- Currently it is in beta, help with your feedback for improvements.
		- It always operates on latest version, specific version is not supported.`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "w, whoami",
			Usage: "To know which version is currently active",
		},
		cli.BoolFlag{
			Name:  "r, refresh",
			Usage: "To refresh currently active aah codebase version to the latest",
		},
	},
	Action: switchAction,
}

func switchAction(c *cli.Context) error {
	branchName := gitBranchName(libDir("aah"))
	if c.Bool("w") || c.Bool("whoami") {
		return whoami(branchName)
	}

	if c.Bool("r") || c.Bool("refresh") {
		fname := friendlyName(branchName)
		fmt.Printf("Refreshing aah '%s' version ...\n\n", fname)
		_ = refresh(branchName)
		fmt.Printf("aah '%s' version refreshed successfully.\n\n", fname)
		return nil
	}

	return doSwitch(branchName)
}

func whoami(branchName string) error {
	if branchName == releaseBranchName {
		fmt.Printf("You're using aah 'release' version.\n\n")
	} else { // treat every branch as 'edge' version expect branch 'master'.
		fmt.Printf("You're using aah 'edge' version, your feedback is appreciated.\n\n")
	}
	return nil
}

func refresh(branchName string) error {
	for _, lib := range libNames {
		// Refresh the branch codebase
		if err := gitPull(libDir(lib)); err != nil {
			fatalf("Unable to refresh library: %s.%s", lib, versionSeries)
		}
	}

	// Refresh dependencies in grace mode
	fetchAahDeps()

	// Install aah CLI for the currently version
	installAahCLI()

	return nil
}

func doSwitch(branchName string) error {
	var toBranch string
	if branchName == releaseBranchName {
		toBranch = edgeBranchName
	} else {
		toBranch = releaseBranchName
	}

	fmt.Printf("Switching aah version to '%s' ...\n\n", friendlyName(toBranch))

	// Checkout the branch
	for _, lib := range libNames {
		if err := gitCheckout(libDir(lib), toBranch); err != nil {
			fatalf("Error occurred which switching aah version: %s", err)
		}
	}

	_ = refresh(branchName)

	if toBranch == releaseBranchName {
		fmt.Printf("You have successfully switched to aah 'release' version.\n\n")
	} else {
		fmt.Printf("You have successfully switched to aah 'edge' version, your feedback is appreciated.\n\n")
	}
	return nil
}

func friendlyName(branchName string) string {
	if branchName == edgeBranchName {
		return "edge"
	}
	return "release"
}
