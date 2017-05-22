// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aahframework.org/essentials.v0"
)

const aahProjectIdentifier = "aah.project"

var (
	listCmd = &command{
		Name:      "list",
		UsageLine: "aah list",
		Short:     "List all aah projects in GOPATH",
		Long: `aah's list command allows you to view all projects that are
  making use of aah in your GOPATH`,
		Run: listRun,
	}
)

type aahProjectDirectories []string

func (a aahProjectDirectories) String() string {

	var formatted string

	for _, v := range a {
		formatted = fmt.Sprintf("%s \n %s", formatted, v)
	}

	return formatted
}

func listRun(args []string) {

	projectsDir := aahProjectDirectories{}

	ess.Walk(gopath, func(path string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if isAahProject(path) {
			projectsDir = append(projectsDir, filepath.Dir(path))
		}

		return nil
	})

	if count := len(projectsDir); count > 0 {
		fmt.Printf("\n %d aah projects were found in your GOPATH \n", count)
		fmt.Println(projectsDir)
		return
	}

	fmt.Println(`No aah projects was found in your GOPATH..
    \n You can create one with aah new`)

}

func isAahProject(file string) bool {
	return strings.HasSuffix(file, aahProjectIdentifier)
}
