// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	"aahframe.work/console"
)

const (
	aahProjectIdentifier = "aah.project"
	goModIdentifier      = "go.mod"
)

var listCmd = console.Command{
	Name:    "list",
	Aliases: []string{"l"},
	Usage:   "Lists all the aah projects",
	Description: `Command 'list' helps you to view all the aah application projects on your System.

	Note: aah CLI is only aware of projects created using 'aah new' otherwise you have to teach 
	it using 'aah list --scan /base/dir/to/scan/aah-projects'.
	`,
	Flags: []console.Flag{
		console.StringFlag{
			Name:  "s, scan",
			Usage: "Directory path to scan for aah projects",
		},
	},
	Action: listAction,
}

func listAction(c *console.Context) error {
	cliLog = initCLILogger(nil)

	scanDir := c.String("scan")
	if len(scanDir) > 0 {
		if !filepath.IsAbs(scanDir) {
			logFatal("Absolute directory path required for scanning")
		}
		scanProjects2Inventory(scanDir)
	}

	if count := len(aahInventory.Projects); count > 0 {
		cliLog.Infof("%d aah projects were found, import paths are: ", count)
		l, ll := 0, 0
		for _, m := range aahInventory.Projects {
			pl := len(m.Path)
			if pl > l {
				l = pl
			}
			if ml := pl + len(m.Dir); ml > ll {
				ll = ml
			}
		}
		fmtStr := "    %-" + strconv.Itoa(l) + "s %s\n"
		fmt.Printf(fmtStr, "Import Path", "Location")
		fmt.Println("    " + chr2str("-", ll-4))
		for _, m := range aahInventory.Projects {
			fmt.Printf(fmtStr, m.Path, m.Dir)
		}
		return nil
	}

	cliLog.Info("No aah projects was found, you can create one with 'aah new'.")
	return nil
}
