// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"aahframework.org/aah.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

var levelNameToLevel = map[string]log.Level{
	"ERROR": log.LevelError,
	"WARN":  log.LevelWarn,
	"INFO":  log.LevelInfo,
	"DEBUG": log.LevelDebug,
	"TRACE": log.LevelTrace,
}

func toLogLevel(l string) log.Level {
	if level, found := levelNameToLevel[strings.ToUpper(l)]; found {
		return level
	}
	return log.LevelInfo
}

func importPathRelwd() string {
	pwd, _ := os.Getwd()
	importPath, _ := filepath.Rel(gosrcDir, pwd)
	return filepath.ToSlash(importPath)
}

// loadAahProjectFile method loads build config from 'aah.project'
func loadAahProjectFile(baseDir string) (*config.Config, error) {
	// read build config from 'aah.project'
	aahProjectFile := filepath.Join(baseDir, "aah.project")
	if !ess.IsFileExists(aahProjectFile) {
		log.Fatal("Missing 'aah.project' file, not a valid aah framework application.")
	}

	log.Infof("Loading aah project file: %s", aahProjectFile)
	return config.LoadFile(aahProjectFile)
}

func getNonEmptyAbsPath(patha, pathb string) string {
	v := firstNonEmpty(patha, pathb)
	if ess.IsStrEmpty(v) {
		return v
	}

	configPath, err := filepath.Abs(v)
	if err != nil {
		log.Fatal(err)
	}

	return configPath
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if !ess.IsStrEmpty(v) {
			return v
		}
	}
	return ""
}

// getAppVersion method returns the aah application version, which used to display
// version from compiled bnary
// 		$ appname version
//
// Application version value priority are -
// 		1. Env variable - AAH_APP_VERSION
// 		2. git describe
// 		3. version number from aah.project file
func getAppVersion(appBaseDir string, cfg *config.Config) string {
	// From env variable
	if version := os.Getenv("AAH_APP_VERSION"); !ess.IsStrEmpty(version) {
		return version
	}

	// fallback version number from file aah.project
	version := cfg.StringDefault("build.version", "")

	// git describe
	if gitcmd, err := exec.LookPath("git"); err == nil {
		appGitDir := filepath.Join(appBaseDir, ".git")
		if !ess.IsFileExists(appGitDir) {
			return version
		}

		_ = os.Chdir(appBaseDir)
		gitArgs := []string{fmt.Sprintf("--git-dir=%s", appGitDir), "describe", "--always", "--dirty"}
		output, err := execCmd(gitcmd, gitArgs, false)
		if err != nil {
			return version
		}

		version = strings.TrimSpace(output)
	}

	return version
}

// getBuildDate method returns application build date, which used to display
// version from compiled bnary
// 		$ appname version
//
// Application build date value priority are -
// 		1. Env variable - AAH_APP_BUILD_DATE
// 		2. Created with time.Now().Format(time.RFC3339)
func getBuildDate() string {
	// From env variable
	if buildDate := os.Getenv("AAH_APP_BUILD_DATE"); !ess.IsStrEmpty(buildDate) {
		return buildDate
	}

	return time.Now().Format(time.RFC3339)
}

func execCmd(cmdName string, args []string, stdout bool) (string, error) {
	cmd := exec.Command(cmdName, args...)
	log.Debug("Executing ", strings.Join(cmd.Args, " "))

	if stdout {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
		_ = cmd.Wait()
	} else {
		bytes, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("\n%s\n%s", string(bytes), err)
		}

		return string(bytes), nil
	}

	return "", nil
}

func renderTmpl(w io.Writer, text string, data interface{}) error {
	tmpl := template.Must(template.New("").Parse(text))
	return tmpl.Execute(w, data)
}

// appBinaryFile method binary file path creation
func appBinaryFile(buildCfg *config.Config, appBuildDir string) string {
	appName := strings.Replace(aah.AppName(), " ", "_", -1)
	appBinaryName := buildCfg.StringDefault("build.binary_name", appName)
	if isWindowsOS() {
		appBinaryName += ".exe"
	}

	return filepath.Join(appBuildDir, "bin", appBinaryName)
}

func addTargetBuildInfo(name string) string {
	if goos := getGOOS(); !ess.IsStrEmpty(goos) {
		name += "-" + strings.ToLower(goos)
	}
	if goarch := getGOARCH(); !ess.IsStrEmpty(goarch) {
		name += "-" + strings.ToLower(goarch)
	}
	return name
}

func isWindowsOS() bool {
	return getGOOS() == "windows"
}

func getGOOS() string {
	goos := os.Getenv("GOOS")
	if ess.IsStrEmpty(goos) {
		goos = runtime.GOOS
	}
	return goos
}

func getGOARCH() string {
	goarch := os.Getenv("GOARCH")
	if ess.IsStrEmpty(goarch) {
		goarch = runtime.GOARCH
	}
	return goarch
}
