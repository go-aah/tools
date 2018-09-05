// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
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
	Path     string     `json:"path"`
	Version  string     `json:"version,omitempty"`
	Time     *time.Time `json:"time,omitempty"`
	Main     bool       `json:"main,omitempty"`
	Indirect bool       `json:"indirect,omitempty"`
	Dir      string     `json:"dir"`
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
	if m := inv.Lookup(importPath); m != nil {
		return fmt.Errorf("aah project '%s' already exists at %s", m.Path, m.Dir)
	}
	inv.Projects = append(inv.Projects, &module{Path: importPath, Dir: dir})
	inv.Persist()
	inv.SortProjects()
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
	inv.Projects = append(inv.Projects[:f], inv.Projects[f+1:]...)
	inv.Persist()
	inv.SortProjects()
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
	if len(aahInventory.Projects) > 0 {
		return
	}
	cliLog.Info("Creating aah projects inventory from GOPATH(s), its an one-time activity")
	gopaths := filepath.SplitList(build.Default.GOPATH)
	for _, gp := range gopaths {
		srcDir := filepath.Join(gp, "src")
		cliLog.Infof("Scanning GOPATH: %s\n", filepath.Join(gp, "..."))
		prefix := srcDir + string(filepath.Separator)
		_ = filepath.Walk(gosrcDir, func(path string, info os.FileInfo, err error) error {
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
				importPath := filepath.ToSlash(strings.TrimPrefix(dir, prefix))
				if !strings.Contains(importPath, "testdata") {
					_ = aahInventory.AddProject(importPath, dir)
				}
			}
			return nil
		})
	}
	aahInventory.Persist()
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
	inv.SortProjects()
	return inv
}
