// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aahframework.org/aah.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

func importPathRelwd() string {
	pwd, _ := os.Getwd()
	importPath, _ := filepath.Rel(gosrcDir, pwd)
	return filepath.ToSlash(importPath)
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

func firstNonEmpty(a, b string) string {
	r := a
	if !ess.IsStrEmpty(b) {
		r = b
	}
	return r
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
	version := cfg.StringDefault("version", "")

	// git describe
	if gitcmd, err := exec.LookPath("git"); err == nil {
		appGitDir := filepath.Join(appBaseDir, ".git")
		if !ess.IsFileExists(appGitDir) {
			return version
		}

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
	log.Info("Executing ", strings.Join(cmd.Args, " "))

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

func renderTmpl(w io.Writer, text string, data interface{}) {
	tmpl := template.Must(template.New("").Parse(text))
	if err := tmpl.Execute(w, data); err != nil {
		log.Fatalf("Unable to render template text: %s", err)
	}
}

// createAppBinaryName method binary name creation
func createAppBinaryName(buildCfg *config.Config) string {
	name := strings.Replace(buildCfg.StringDefault("name", aah.AppName()), " ", "_", -1)
	appBinaryName := buildCfg.StringDefault("build.binary_name", name)
	if isWindows {
		appBinaryName += ".exe"
	}

	return filepath.Join(gopath, "bin", "aah.d", aah.AppImportPath(), appBinaryName)
}
