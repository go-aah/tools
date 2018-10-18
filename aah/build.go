// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"aahframe.work"
	"aahframe.work/config"
	"aahframe.work/console"
	"aahframe.work/essentials"
	"aahframe.work/log"
)

var buildCmd = console.Command{
	Name:    "build",
	Aliases: []string{"b"},
	Usage:   "Builds aah application for deployment (single or non-single binary)",
	Description: `Builds aah application for deployment. It supports single and non-single
	binary. It is a trade-off learn more https://docs.aahframework.org/vfs.html

	Artifact naming convention:  <appbinaryname>-<appversion>-<goos>-<goarch>.zip
	For e.g.: aahwebsite-381eaa8-darwin-amd64.zip

	Examples of short and long flags:
    aah build  OR  aah b
		aah build --single  OR  aah b -s
    aah build -o /Users/jeeva -s
		aah build -o /Users/jeeva/aahwebsite.zip`,
	Flags: []console.Flag{
		console.StringFlag{
			Name:  "o, output",
			Usage: "Output of aah application build artifact; the default is '<appbasedir>/build/<appbinaryname>-<appversion>-<goos>-<goarch>.zip'",
		},
		console.BoolFlag{
			Name:  "s, single",
			Usage: "Creates aah single application binary",
		},
	},
	Action: buildAction,
}

func buildAction(c *console.Context) error {
	if !isAahProject() {
		logFatalf("Please go to aah application base directory and run '%s'.", strings.Join(os.Args, " "))
	}

	importPath := appImportPath(c)
	if ess.IsStrEmpty(importPath) {
		logFatalf("Unable to infer import path, ensure you're in the application base directory")
	}
	chdirIfRequired(importPath)
	if err := aah.Init(importPath); err != nil {
		logFatal(err)
	}

	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)

	cliLog.Infof("Loaded aah project file: %s", filepath.Join(aah.AppBaseDir(), aahProjectIdentifier))
	cliLog.Infof("Build starts for '%s' [%s]", aah.AppName(), aah.AppImportPath())

	if c.Bool("s") || c.Bool("single") {
		buildSingleBinary(c, projectCfg)
	} else {
		buildBinary(c, projectCfg)
	}

	return nil
}

func buildBinary(c *console.Context, projectCfg *config.Config) {
	appBaseDir := aah.AppBaseDir()
	processVFSConfig(projectCfg, false)

	appBinary, err := compileApp(&compileArgs{
		Cmd:        "BuildCmd",
		ProjectCfg: projectCfg,
		AppPack:    true,
	})
	if err != nil {
		logFatal(err)
	}

	buildBaseDir, err := copyFilesToWorkingDir(projectCfg, appBaseDir, appBinary)
	if err != nil {
		logFatal(err)
	}

	destArchiveFile := createZipArchiveName(c, projectCfg, appBaseDir, appBinary)

	// Creating app archive
	if err = createZipArchive(buildBaseDir, destArchiveFile); err != nil {
		logFatal(err)
	}

	cliLog.Infof("Build successful for '%s' [%s]", aah.AppName(), aah.AppImportPath())
	cliLog.Infof("Application artifact is here: %s\n", destArchiveFile)
}

func buildSingleBinary(c *console.Context, projectCfg *config.Config) {
	cliLog.Infof("Embed starts for '%s' [%s]", aah.AppName(), aah.AppImportPath())
	processVFSConfig(projectCfg, true)
	cliLog.Infof("Embed successful for '%s' [%s]", aah.AppName(), aah.AppImportPath())

	appBinary, err := compileApp(&compileArgs{
		Cmd:        "BuildCmd",
		ProjectCfg: projectCfg,
		AppPack:    true,
		AppEmbed:   true,
	})
	if err != nil {
		logFatal(err)
	}

	// Creating app archive
	destArchiveFile := createZipArchiveName(c, projectCfg, aah.AppBaseDir(), appBinary)
	if err = createZipArchive(appBinary, destArchiveFile); err != nil {
		logFatal(err)
	}

	cliLog.Infof("Build successful for '%s' [%s]", aah.AppName(), aah.AppImportPath())
	cliLog.Infof("Application artifact is here: %s\n", destArchiveFile)
}

func processVFSConfig(projectCfg *config.Config, mode bool) {
	appBaseDir := aah.AppBaseDir()
	cleanupAutoGenVFSFiles(appBaseDir)

	excludes, _ := projectCfg.StringList("build.excludes")
	noGzipList, _ := projectCfg.StringList("vfs.no_gzip")

	if mode {
		// Default mount point
		if err := processMount(mode, appBaseDir, "/app", appBaseDir, ess.Excludes(excludes), noGzipList); err != nil {
			logFatal(err)
		}
	}

	// Custom mount points
	mountKeys := projectCfg.KeysByPath("vfs.mount")
	for _, key := range mountKeys {
		vroot := projectCfg.StringDefault("vfs.mount."+key+".mount_path", "")
		proot := projectCfg.StringDefault("vfs.mount."+key+".physical_path", "")

		if !filepath.IsAbs(proot) {
			logErrorf("vfs %s: physical_path is not absolute path, skip mount: %s", proot, vroot)
			continue
		}

		if !ess.IsStrEmpty(vroot) && !ess.IsStrEmpty(proot) {
			if err := processMount(mode, appBaseDir, vroot, proot, ess.Excludes(excludes), noGzipList); err != nil {
				logError(err)
			}
		}
	}
}

func copyFilesToWorkingDir(projectCfg *config.Config, appBaseDir, appBinary string) (string, error) {
	appBinaryName := filepath.Base(appBinary)
	tmpDir, err := ioutil.TempDir("", appBinaryName)
	if err != nil {
		return "", fmt.Errorf("unable to get temp directory: %s", err)
	}

	buildBaseDir := filepath.Join(tmpDir, ess.StripExt(appBinaryName))
	ess.DeleteFiles(buildBaseDir)
	if err = ess.MkDirAll(buildBaseDir, permRWXRXRX); err != nil {
		return "", err
	}

	// binary file
	binDir := filepath.Join(buildBaseDir, "bin")
	_ = ess.MkDirAll(binDir, permRWXRXRX)
	_, _ = ess.CopyFile(binDir, appBinary)

	// apply executable file mode
	if err = ess.ApplyFileMode(filepath.Join(binDir, appBinaryName), permRWXRXRX); err != nil {
		log.Error(err)
	}

	// build package excludes
	cfgExcludes, _ := projectCfg.StringList("build.excludes")
	excludes := ess.Excludes(cfgExcludes)
	if err = excludes.Validate(); err != nil {
		return "", err
	}

	// aah application and custom directories
	appDirs, _ := ess.DirsPath(appBaseDir, false)
	subTreeExcludes := ess.Excludes(excludeAndCreateSlice(cfgExcludes, "app"))
	for _, srcdir := range appDirs {
		if excludes.Match(filepath.Base(srcdir)) {
			continue
		}

		if ess.IsFileExists(srcdir) {
			if err = ess.CopyDir(buildBaseDir, srcdir, subTreeExcludes); err != nil {
				if !strings.HasSuffix(err.Error(), "/bin") {
					return "", err
				}
			}
		}
	}

	return buildBaseDir, err
}

func createZipArchive(buildBaseDir, destArchiveFile string) error {
	ess.DeleteFiles(destArchiveFile)

	archiveBaseDir := filepath.Dir(destArchiveFile)
	if err := ess.MkDirAll(archiveBaseDir, permRWXRXRX); err != nil {
		return err
	}
	return ess.Zip(destArchiveFile, buildBaseDir)
}

func createZipArchiveName(c *console.Context, projectCfg *config.Config, appBaseDir, appBinary string) string {
	var err error
	outputFile := firstNonEmpty(c.String("o"), c.String("output"))
	archiveName := ess.StripExt(filepath.Base(appBinary)) + "-" + getAppVersion(appBaseDir, projectCfg)
	archiveName = addTargetBuildInfo(archiveName)

	var destArchiveFile string
	if ess.IsStrEmpty(outputFile) {
		destArchiveFile = filepath.Join(appBaseDir, "build", archiveName)
	} else {
		destArchiveFile, err = filepath.Abs(outputFile)
		if err != nil {
			logFatal(err)
		}

		if !strings.HasSuffix(destArchiveFile, ".zip") {
			destArchiveFile = filepath.Join(destArchiveFile, archiveName)
		}
	}

	if !strings.HasSuffix(destArchiveFile, ".zip") {
		destArchiveFile = destArchiveFile + ".zip"
	}
	return destArchiveFile
}
