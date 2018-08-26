// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path"

	"gopkg.in/urfave/cli.v1"
)

var updateCmd = cli.Command{
	Name:    "update",
	Aliases: []string{"u"},
	Usage:   "Updates aah to the latest release version on your GOPATH",
	Description: `Provides an easy and convenient way to update your aah framework version
to the latest release version on your GOPATH.

	Examples of short and long flags:
		aah u
		aah update

	Note:
		- Currently it works with only GOPATH.
		- It always operates on aah latest release version, specific version is not supported.
  `,
	Action: updateAction,
}

func updateAction(c *cli.Context) error {
	cliLog = initCLILogger(nil)
	branchName := gitBranchName(libDir("aah"))
	if branchName != releaseBranchName {
		fmt.Printf("Update command only applicable to aah release version.\n")
		fmt.Printf("Currently you're on aah 'edge' version, use 'aah s -r' command to refresh edge version.\n\n")
		return nil
	}

	fmt.Printf("Update aah version to the latest release ...\n\n")
	gocmdName := goCmdName()
	args := []string{"get"}
	if gocmdName == "go" {
		args = append(args, "-u")
	}
	args = append(args, path.Join(aahImportPath, "cli", "aah"))
	if _, err := execCmd(gocmd, args, false); err != nil {
		logFatalf("Unable to update aah to the latest release version: %s", err)
	}

	fmt.Printf("You have successfully updated aah to the latest release version.\n\n")

	return nil
}
