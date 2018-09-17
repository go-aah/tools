// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aahframe.work/aah/essentials"
)

type inventory struct {
	Projects []*module `json:"projects,omitempty"`
}

type module struct {
	Path     string     `json:"path,omitempty"`
	Version  string     `json:"version,omitempty"`
	Time     *time.Time `json:"time,omitempty"`
	Main     bool       `json:"main,omitempty"`
	Indirect bool       `json:"indirect,omitempty"`
	Dir      string     `json:"dir,omitempty"`
	GoMod    string     `json:"go_mod,omitempty"`
}

var aahInventory = loadInventory()

func (inv *inventory) Lookup(importPath string) *module {
	pl := len(inv.Projects)
	i := sort.Search(pl, func(i int) bool {
		return inv.Projects[i].Path >= importPath
	})
	if i < pl && inv.Projects[i].Path == importPath {
		return inv.Projects[i]
	}
	return nil
}

func (inv *inventory) AddProject(importPath, dir string) error {
	if len(importPath) == 0 || len(dir) == 0 {
		return errors.New("missing required inputs to added aah project into inventory")
	}
	if m := inv.Lookup(importPath); m != nil {
		return fmt.Errorf("aah project '%s' already exists at %s", m.Path, m.Dir)
	}
	inv.Projects = append(inv.Projects, &module{Path: importPath, Dir: dir})
	inv.SortProjects()
	inv.Persist()
	return nil
}

func (inv *inventory) DelProject(importPath string) {
	f := -1
	for i, m := range inv.Projects {
		if m.Path == importPath {
			f = i
			break
		}
	}
	if f > -1 {
		inv.Projects = append(inv.Projects[:f], inv.Projects[f+1:]...)
		inv.SortProjects()
		inv.Persist()
	}
}

func (inv *inventory) Persist() {
	inventoryPath := filepath.Join(aahPath(), "inventory")
	f, err := os.OpenFile(inventoryPath, os.O_RDWR|os.O_CREATE, os.FileMode(0644))
	if err != nil {
		logFatalf("Unable to create/open aah projects inventory: %v", err)
	}
	defer ess.CloseQuietly(f)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(inv); err != nil {
		logErrorf("Unable to write aah projects inventory to %s: %v", inventoryPath, err)
	}
}

func (inv *inventory) SortProjects() {
	sort.Slice(inv.Projects, func(i, j int) bool { return inv.Projects[i].Path < inv.Projects[j].Path })
}

func createProjectInventory() {
	cliLog.Info("Creating aah projects inventory from GOPATH(s), its an one-time activity\n")
	for _, gp := range filepath.SplitList(build.Default.GOPATH) {
		scanProjects2Inventory(filepath.Join(gp, "src"))
	}
}

func scanProjects2Inventory(baseDir string) {
	cliLog.Infof("Scanning aah projects on %s...\n", baseDir)
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Skip Git Directory
		if strings.Contains(path, "/.git/") || strings.Contains(path, "\\.git\\") ||
			path[0] == '.' {
			return nil
		}
		if isAahProject(path) {
			dir := filepath.Dir(path)
			if err := os.Chdir(dir); err == nil {
				importPath := appImportPath(nil)
				if !strings.Contains(importPath, "testdata") {
					_ = aahInventory.AddProject(importPath, dir)
				}
			}
		}
		return nil
	})
}

func loadInventory() *inventory {
	inv := new(inventory)
	inventoryPath := filepath.Join(aahPath(), "inventory")
	if !ess.IsFileExists(inventoryPath) {
		return inv
	}
	f, err := os.Open(inventoryPath)
	if err != nil {
		logError(err)
		return inv
	}
	defer ess.CloseQuietly(f)
	if err = json.NewDecoder(f).Decode(inv); err != nil {
		logError(err)
	}

	filter := make([]*module, 0)
	for _, m := range inv.Projects {
		if isAahProject(filepath.Join(m.Dir, aahProjectIdentifier)) {
			filter = append(filter, m)
		}
	}

	if len(inv.Projects) != len(filter) {
		inv.Projects = filter
		inv.SortProjects()
		inv.Persist()
	} else {
		inv.SortProjects()
	}
	return inv
}
