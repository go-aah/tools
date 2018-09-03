// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"gopkg.in/urfave/cli.v1"
)

const aahProjectIdentifier = "aah.project"

var listCmd = cli.Command{
	Name:    "list",
	Aliases: []string{"l"},
	Usage:   "Lists all the aah projects on your GOPATH",
	Description: `Command 'list' helps you to view all the aah application projects on your GOPATH.
	`,
	Action: listAction,
}

func listAction(c *cli.Context) error {
	cliLog = initCLILogger(nil)
	createProjectInventory()

	if count := len(aahInventory.Projects); count > 0 {
		cliLog.Infof("%d aah projects were found, import paths are:\n", count)
		for _, m := range aahInventory.Projects {
			fmt.Printf("    %s\n", m.Path)
		}
		fmt.Println()
		return nil
	}

	cliLog.Info("No aah projects was found, you can create one with 'aah new'\n")
	return nil
}
