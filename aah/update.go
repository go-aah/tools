// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
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
	Usage:   "Update your aah to the latest release version on your GOPATH",
	Description: `Provides an easy and convenient way to update your aah to the latest release version on your GOPATH.

	Examples of short and long flags:
		aah u
		aah update

	Note:
		- Currently it works with only GOPATH. Gradually I will add vendorize support too.
		- It always operates on aah latest release version, specific version is not supported.
  `,
	Action: updateAction,
}

func updateAction(c *cli.Context) error {
	branchName := gitBranchName(libDir("aah"))
	if branchName != releaseBranchName {
		fmt.Printf("Update command only applicable to aah release version.\n")
		fmt.Printf("Currently you're on aah 'edge' version, use 'aah switch --refresh' command to get latest edge version.\n\n")
		return nil
	}

	fmt.Printf("Update aah version to the latest release ...\n\n")

	args := []string{"get", "-u", path.Join(importPrefix, "tools.v0", "aah")}
	if _, err := execCmd(gocmd, args, false); err != nil {
		fatalf("Unable to update aah to the latest release version: %s", err)
	}

	fmt.Printf("You have successfully updated aah to the latest release version.\n\n")

	return nil
}
