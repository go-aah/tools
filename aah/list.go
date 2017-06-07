// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"

	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

const aahProjectIdentifier = "aah.project"

var (
	listCmd = &command{
		Name:      "list",
		UsageLine: "aah list",
		Short:     "list all aah projects in GOPATH",
		Long: `
		List command allows you to view all projects that are making use of aah in your GOPATH.
		`,
	}
)

func listRun(args []string) {
	_ = log.SetPattern("%message")
	log.Infof("Scanning GOPATH: %s", filepath.Join(gopath, "..."))
	log.Info()

	var aahProjects []string
	_ = ess.Walk(gopath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if isAahProject(path) {
			aahProjects = append(aahProjects, filepath.Dir(path))
		}

		return nil
	})

	if count := len(aahProjects); count > 0 {
		log.Infof("%d aah projects were found, import paths are-", count)
		prefix := gosrcDir + string(filepath.Separator)
		for _, p := range aahProjects {
			log.Infof("    %s", filepath.ToSlash(strings.TrimPrefix(p, prefix)))
		}
		log.Info()
		return
	}

	log.Info(`No aah projects was found, you can create one with 'aah new'`)
	log.Info()
	_ = log.SetPattern(log.DefaultPattern)
}

func init() {
	listCmd.Run = listRun
}
