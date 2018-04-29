// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/urfave/cli.v1"
)

const (
	releaseBranchName = "master"
	edgeBranchName    = "v0-edge"
	emojiThumpsUp     = `👍`
)

var switchCmd = cli.Command{
	Name:    "switch",
	Aliases: []string{"s"},
	Usage:   "Switches between aah release and edge version",
	Description: `Provides an ability to switch between aah release and latest edge version.

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
		- Currently it works with only GOPATH.
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
		cliLog.Infof("Refresh option is for 'edge' version only, currently you're on '%s' version.\n", fname)
		cliLog.Infof("Use 'aah update' command to update your aah to the latest release version on your GOPATH.\n")
		return nil
	}

	cliLog.Infof("Refreshing aah '%s' version ...\n", fname)

	// Refresh to latest edge codebase
	refreshLibCode(aahLibraryDirs())

	// Refresh dependencies in grace mode
	fetchLibDeps()

	// Install aah CLI for current version
	installCLI()

	cliLog.Infof("You have successfully refreshed aah '%s' version.\n", fname)
	return nil
}

func doSwitch(branchName, target string) error {
	fname := friendlyName(branchName)
	if target == fname {
		cliLog.Infof("Currently you're on aah '%s' version.\n", fname)
		cliLog.Infof("To switch to release version. Run 'aah s -v release'\n")

		if fname == "edge" {
			ans := collectYesOrNo(reader, "Would you like to refresh 'edge' to latest updates? ([Y]es or [N]o), default is 'N'")
			fmt.Println()
			if ans {
				doRefresh(branchName)
			}
		}

		return nil
	}

	var toBranch string
	if branchName == releaseBranchName {
		toBranch = edgeBranchName
	} else {
		toBranch = releaseBranchName
	}

	cliLog.Infof("Switching aah to '%s' version ...\n", friendlyName(toBranch))

	// Checkout the branch
	aahLibDirs := aahLibraryDirs()
	var wg sync.WaitGroup
	for _, dir := range aahLibDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			baseName := filepath.Base(d)
			if err := gitCheckout(d, toBranch); err != nil {
				logErrorf("Unable to switch library version, possibliy you may have local changes[%s]: %s", baseName, err)
			}
			cliLog.Tracef("Library '%s' have been switched to '%s' successfully", baseName, toBranch)
		}(dir)
	}
	wg.Wait()

	if toBranch == edgeBranchName {
		cliLog.Infof("Refreshing aah to latest '%s' updates ...\n", friendlyName(toBranch))
		refreshLibCode(aahLibDirs)
	}

	// Refresh dependencies in grace mode
	fetchLibDeps()

	// Install aah CLI for current version
	installCLI()

	if toBranch == releaseBranchName {
		cliLog.Infof("You have successfully switched %s.\n", emojiThumpsUp)
	} else {
		cliLog.Infof("You have successfully switched %s, your feedback is appreciated.\n", emojiThumpsUp)
	}
	return nil
}

func friendlyName(branchName string) string {
	if branchName == edgeBranchName {
		return "edge"
	}
	return "release"
}
