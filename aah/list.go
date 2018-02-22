// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"aahframework.org/essentials.v0"
)

const aahProjectIdentifier = "aah.project"

var listCmd = cli.Command{
	Name:    "list",
	Aliases: []string{"l"},
	Usage:   "List all aah projects in GOPATH",
	Description: `List command allows you to view all projects that are making use of aah in your GOPATH.
	`,
	Action: listAction,
}

func listAction(c *cli.Context) error {
	fmt.Println("Scanning GOPATH:", filepath.Join(gopath, "..."))
	fmt.Println()

	var aahProjects []string
	_ = ess.Walk(gosrcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip Git Directory
		if strings.Contains(path, "/.git/") || strings.Contains(path, "\\.git\\") {
			return nil
		}

		if isAahProject(path) {
			aahProjects = append(aahProjects, filepath.Dir(path))
		}

		return nil
	})

	if count := len(aahProjects); count > 0 {
		fmt.Printf("%d aah projects were found, import paths are:\n", count)
		prefix := gosrcDir + string(filepath.Separator)
		for _, p := range aahProjects {
			fmt.Printf("    %s\n", filepath.ToSlash(strings.TrimPrefix(p, prefix)))
		}
		fmt.Println()
		return nil
	}

	fmt.Println(`No aah projects was found, you can create one with 'aah new'`)
	fmt.Println()
	return nil
}
