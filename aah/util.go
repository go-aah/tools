// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

func importPathRelwd() string {
	pwd, _ := os.Getwd()
	var importPath string
	if strings.HasPrefix(pwd, gosrcDir) {
		importPath, _ = filepath.Rel(gosrcDir, pwd)
	} else if idx := strings.Index(pwd, "src"); idx > 0 {
		importPath = pwd[idx+4:]
	}
	return filepath.ToSlash(importPath)
}

// loadAahProjectFile method loads build config from 'aah.project'
func loadAahProjectFile(baseDir string) (*config.Config, error) {
	aahProjectFile := filepath.Join(baseDir, aahProjectIdentifier)
	if !ess.IsFileExists(aahProjectFile) {
		fatal("Missing 'aah.project' file, not a valid aah framework application.")
	}
	return config.LoadFile(aahProjectFile)
}

func getNonEmptyAbsPath(patha, pathb string) string {
	v := firstNonEmpty(patha, pathb)
	if ess.IsStrEmpty(v) {
		return v
	}

	configPath, err := filepath.Abs(v)
	if err != nil {
		fatal(err)
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
		if !ess.IsFileExists(filepath.Join(appBaseDir, ".git")) {
			return version
		}

		gitArgs := []string{"-C", appBaseDir, "describe", "--always", "--dirty"}
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
	log.Trace("Executing ", strings.Join(cmd.Args, " "))

	if stdout {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
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

func excludeAndCreateSlice(arr []string, str string) []string {
	var result []string
	for _, v := range arr {
		if str == v {
			continue
		}
		result = append(result, v)
	}
	return result
}

func isAahProject(file string) bool {
	return strings.HasSuffix(file, aahProjectIdentifier)
}

func findAvailablePort() string {
	lstn, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Error(err)
		return "0"
	}
	defer ess.CloseQuietly(lstn)

	return strconv.Itoa(lstn.Addr().(*net.TCPAddr).Port)
}

func initLogger(cfg *config.Config) {
	logCfg, _ := config.ParseString("")
	logCfg.SetString("log.receiver", "console")
	logCfg.SetString("log.level", cfg.StringDefault("log.level", "info"))
	logCfg.SetBool("log.color", cfg.BoolDefault("log.color", true))

	cliLog, _ := log.New(logCfg)
	log.SetDefaultLogger(cliLog)
}

func gitCheckout(dir, branch string) error {
	if gitcmd, err := exec.LookPath("git"); err == nil {
		gitArgs := []string{"-C", dir, "checkout", branch}
		_, err := execCmd(gitcmd, gitArgs, false)
		return err
	}
	return nil
}

func libImportPath(name string) string {
	return fmt.Sprintf("%s/%s.%s", importPrefix, name, versionSeries)
}

func libDir(name string) string {
	importPath := libImportPath(name)
	return filepath.FromSlash(filepath.Join(gopath, "src", importPath))
}

func gitBranchName(dir string) string {
	if !ess.IsDir(dir) {
		log.Tracef("Given path '%s' is not a directory", dir)
		return ""
	}

	if gitcmd, err := exec.LookPath("git"); err == nil {
		gitArgs := []string{"-C", dir, "rev-parse", "--abbrev-ref", "HEAD"}
		output, _ := execCmd(gitcmd, gitArgs, false)
		return strings.TrimSpace(output)
	}
	return ""
}

func gitPull(dir string) error {
	if gitcmd, err := exec.LookPath("git"); err == nil {
		gitArgs := []string{"-C", dir, "pull"}
		_, err := execCmd(gitcmd, gitArgs, false)
		return err
	}
	return nil
}

func enableGitRedirects() {
	if gitcmd, err := exec.LookPath("git"); err == nil {
		gitArgs := []string{"config", "--global", "http.https://aahframework.org.followRedirects", "true"}
		_, _ = execCmd(gitcmd, gitArgs, false)
		gitArgs = []string{"config", "--global", "http.https://gopkg.in.followRedirects", "true"}
		_, _ = execCmd(gitcmd, gitArgs, false)
	}
}

func goGet(pkgs ...string) error {
	for _, pkg := range pkgs {
		args := []string{"get", pkg}
		if _, err := execCmd(gocmd, args, false); err != nil {
			return err
		}
	}
	return nil
}
