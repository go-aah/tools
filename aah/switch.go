// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"strings"

	"gopkg.in/urfave/cli.v1"
)

const (
	releaseBranchName = "master"
	edgeBranchName    = "v0-edge"
)

var switchCmd = cli.Command{
	Name:    "switch",
	Aliases: []string{"s"},
	Usage:   "Switch between aah release and edge version",
	Description: `Provides an ability to switch between aah release (currently on your GOPATH) and latest edge version.

	Examples of short and long flags:
		aah s
		aah switch

	To check which version is currently active:
		aah s -w
		aah switch --whoami

	To refresh edge version to the latest codebase:
		aah s -r
		aah switch --refresh

	Note:
		- Currently it works with only GOPATH. Gradually I will add vendorize support too.
		- It always operates on latest edge version and current release version on your GOPATH, specific version is not supported.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "v, version",
			Usage: "To mention latest release or edge version",
			Value: "edge",
		},
		cli.BoolFlag{
			Name:  "w, whoami",
			Usage: "To know which version is currently active",
		},
		cli.BoolFlag{
			Name:  "r, refresh",
			Usage: "To refresh edge version to the latest codebase",
		},
	},
	Action: switchAction,
}

func switchAction(c *cli.Context) error {
	cliLog = initCLILogger(nil)
	branchName := gitBranchName(libDir("aah"))
	if c.Bool("w") || c.Bool("whoami") {
		return whoami(branchName)
	}

	if c.Bool("r") || c.Bool("refresh") {
		return doRefresh(branchName)
	}

	return doSwitch(branchName, strings.ToLower(firstNonEmpty(c.String("v"), c.String("version"))))
}

func whoami(branchName string) error {
	if branchName == releaseBranchName {
		cliLog.Infof("You're using aah 'release' version.\n")
	} else { // treat every branch as 'edge' version expect branch 'master'.
		cliLog.Infof("You're using aah 'edge' version, your feedback is highly appreciated.\n")
	}
	return nil
}

func doRefresh(branchName string) error {
	fname := friendlyName(branchName)
	if branchName == releaseBranchName {
		cliLog.Infof("Refresh is only applicable to edge version, currently you're on '%s' version.\n", fname)
		cliLog.Infof("Use 'aah update' command to update your aah to the latest release version on your GOPATH.\n")
		return nil
	}

	cliLog.Infof("Refreshing aah '%s' version ...\n", fname)

	// Refresh to latest edge codebase
	refreshCodebase(libNames...)

	// Refresh dependencies in grace mode
	fetchAahDeps()

	// Install aah CLI for the currently version
	installAahCLI()

	cliLog.Infof("You have successfully refreshed aah '%s' version.\n", fname)
	return nil
}

func doSwitch(branchName, target string) error {
	fname := friendlyName(branchName)
	if target == fname {
		cliLog.Infof("You're already on '%s' version.\n", fname)
		cliLog.Infof("To switch to latest release version. Run 'aah switch -v release'\n")
		return nil
	}

	var toBranch string
	if branchName == releaseBranchName {
		toBranch = edgeBranchName
	} else {
		toBranch = releaseBranchName
	}

	cliLog.Infof("Switching aah version to '%s' ...\n", friendlyName(toBranch))

	// Checkout the branch
	for _, lib := range libNames {
		if err := gitCheckout(libDir(lib), toBranch); err != nil {
			logFatalf("Error occurred which switching aah version: %s", err)
		}
	}

	if toBranch == edgeBranchName {
		cliLog.Infof("Refreshing aah version to latest '%s' ...\n", fname)
		refreshCodebase(libNames...)
	}

	// Refresh dependencies in grace mode
	fetchAahDeps()

	// Install aah CLI for the currently version
	installAahCLI()

	if toBranch == releaseBranchName {
		cliLog.Infof("You have successfully switched to aah 'release' version.\n")
	} else {
		cliLog.Infof("You have successfully switched to aah 'edge' version, your feedback is appreciated.\n")
	}
	return nil
}

func friendlyName(branchName string) string {
	if branchName == edgeBranchName {
		return "edge"
	}
	return "release"
}
