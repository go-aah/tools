// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/radovskyb/watcher.v1"
	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0-unstable"
	"aahframework.org/log.v0-unstable"
)

var runCmd = cli.Command{
	Name:    "run",
	Aliases: []string{"r"},
	Usage:   "Run aah framework application",
	Description: `Run the aah framework web/api application.

	Examples of short and long flags:
    aah run
		aah run -p qa

		aah run -i github.com/user/appname
		aah run -i github.com/user/appname -p qa
		aah run -i github.com/user/appname -c /path/to/config/external.conf -p qa

    aah run --importpath github.com/username/name
		aah run --importpath github.com/username/name --profile qa
		aah run --importpath github.com/username/name --config /path/to/config/external.conf --profile qa

	Note: It is recommended to use build and deploy approach instead of
	using 'aah run' for production use.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "i, importpath",
			Usage: "Import path of aah application",
		},
		cli.StringFlag{
			Name:  "p, profile",
			Usage: "Environment profile name to activate. e.g: dev, qa, prod",
			Value: "dev",
		},
		cli.StringFlag{
			Name:  "c, config",
			Usage: "External config for overriding aah.conf values",
		},
	},
	Action: runAction,
}

func runAction(c *cli.Context) error {
	importPath := firstNonEmpty(c.String("i"), c.String("importpath"))
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		fatalf("Given import path '%s' does not exists", importPath)
	}

	appStartArgs := []string{}
	configPath := getNonEmptyAbsPath(c.String("c"), c.String("config"))
	if !ess.IsStrEmpty(configPath) {
		appStartArgs = append(appStartArgs, "-config", configPath)
	}

	envProfile := firstNonEmpty(c.String("p"), c.String("config"))
	if !ess.IsStrEmpty(envProfile) {
		appStartArgs = append(appStartArgs, "-profile", envProfile)
	}

	inst := make(chan bool)
	watch := make(chan bool)

SA:
	aah.Init(importPath)

	projectCfg, err := loadAahProjectFile(aah.AppBaseDir())
	if err != nil {
		fatalf("aah project file error: %s", err)
	}

	_ = log.SetLevel(projectCfg.StringDefault("build.log_level", "info"))

	appBinary, err := compileApp(projectCfg, false)
	if err != nil {
		fatal(err)
	}

	w := watcher.New()
	go startWatcher(projectCfg, aah.AppBaseDir(), w, watch)
	go startApp(appBinary, appStartArgs, inst)

	// Wait for application changes
	<-watch
	inst <- true

	// Changes detected give some grace time before proceeding
	time.Sleep(time.Millisecond * 100)
	w.Close()
	goto SA
}

func startApp(appBinary string, args []string, inst <-chan bool) {
	cmd := exec.Command(appBinary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatal(err)
	}

	// wait for Shutdown instruction
	for {
		if <-inst {
			if isWindowsOS() {
				_ = cmd.Process.Kill()
			} else {
				_ = cmd.Process.Signal(os.Interrupt)
			}
			return
		}
	}
}

func startWatcher(projectCfg *config.Config, baseDir string, w *watcher.Watcher, watch chan<- bool) {
	w.IgnoreHiddenFiles(true)
	w.SetMaxEvents(1)

	loadWatchFiles(projectCfg, baseDir, w)

	go func() { w.Wait() }()

	go func() {
		for {
			select {
			case e := <-w.Event:
				switch e.Op {
				case watcher.Create:
					log.Info("Adding file to watch list:", e.Path)
					if err := w.Add(e.Path); err != nil {
						log.Error(err)
					}
				default:
					log.Info("Application file change(s) detected")
					watch <- true
				}
			case err := <-w.Error:
				if err == watcher.ErrWatchedFileDeleted {
					// treat as information, not an error
					log.Info("Watched file/directory is deleted, just move on")
				}
			case <-w.Closed:
				return
			}
		}
	}()

	if log.IsLevelTrace() {
		var fileList []string
		for path := range w.WatchedFiles() {
			fileList = append(fileList, stripGoPath(path))
		}
		log.Trace("Watched files:\n\t", strings.Join(fileList, "\n\t"))
	}

	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Error(err)
	}
}

func loadWatchFiles(projectCfg *config.Config, baseDir string, w *watcher.Watcher) {
	// standard file ignore list for aah project
	stdIgnoreList := []string{
		filepath.Join(baseDir, aah.AppName()+".pid"),
		filepath.Join(baseDir, "app", "aah.go"),
	}

	// user can provide their list via config
	dirExcludes, _ := projectCfg.StringList("watch.dir_excludes")
	if len(dirExcludes) == 0 { // put defaults
		dirExcludes = append(dirExcludes, ".*")
	}

	fileExcludes, _ := projectCfg.StringList("watch.file_excludes")
	if len(fileExcludes) == 0 { // put defaults
		fileExcludes = append(fileExcludes, ".*", "_test.go", "LICENSE", "README.md")
	}

	// standard dir ignore list for aah project
	dirExcludes = append(dirExcludes, "build", "static", "vendor", "tests", "logs")

	dirs, _ := ess.DirsPathExcludes(baseDir, true, dirExcludes)
	for _, d := range dirs {
		if err := w.Add(d); err != nil {
			log.Errorf("Unable add watch for '%v'", d)
		}

		files, _ := ess.FilesPathExcludes(d, false, fileExcludes)
		for _, f := range files {
			if err := w.Add(f); err != nil {
				log.Errorf("Unable add watch for '%v'", f)
			}
		}
	}

	// Add ignore list
	if err := w.Ignore(stdIgnoreList...); err != nil {
		log.Error(err)
	}
}
