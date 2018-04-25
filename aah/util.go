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

	"aahframework.org/aah.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
	"gopkg.in/urfave/cli.v1"
)

func importPathRelwd() string {
	pwd, _ := os.Getwd()

	var importPath string
	if idx := strings.Index(pwd, "src"); idx > 0 {
		srcDir := pwd[:idx+3]
		appDir := pwd
		for {
			if ess.IsFileExists(filepath.Join(appDir, aahProjectIdentifier)) {
				importPath, _ = filepath.Rel(srcDir, appDir)
				break
			} else {
				appDir = filepath.Dir(appDir)
			}

			if appDir == srcDir {
				break
			}
		}
	}

	return filepath.ToSlash(importPath)
}

func aahProjectCfg(baseDir string) *config.Config {
	projectFile := filepath.Join(baseDir, aahProjectIdentifier)
	if !ess.IsFileExists(projectFile) {
		logFatal("Missing 'aah.project' file, not a valid aah framework application.")
	}

	cfg, err := config.LoadFile(projectFile)
	if err != nil {
		logFatalf("aah project file error: %s", err)
	}
	return cfg
}

func getNonEmptyAbsPath(patha, pathb string) string {
	v := firstNonEmpty(patha, pathb)
	if ess.IsStrEmpty(v) {
		return v
	}

	configPath, err := filepath.Abs(v)
	if err != nil {
		logFatal(err)
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
	cliLog.Trace("Executing ", strings.Join(cmd.Args, " "))

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
	tmpl := template.Must(template.New("").Funcs(appTemplateFuncs).Parse(text))
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
		logError(err)
		return "0"
	}
	defer ess.CloseQuietly(lstn)

	return strconv.Itoa(lstn.Addr().(*net.TCPAddr).Port)
}

func initCLILogger(cfg *config.Config) *log.Logger {
	if cfg == nil {
		cfg, _ = config.ParseString("")
	}

	printDeprecateInfo := false
	logLevel := cfg.StringDefault("log.level", "info")
	if level, found := cfg.String("build.log_level"); found {
		logLevel = level
		printDeprecateInfo = true
	}

	logCfg, _ := config.ParseString("")
	logCfg.SetString("log.receiver", "console")
	logCfg.SetString("log.level", logLevel)
	logCfg.SetString("log.pattern", "%message")
	logCfg.SetBool("log.color", cfg.BoolDefault("log.color", true))
	l, _ := log.New(logCfg)

	if printDeprecateInfo {
		// DEPRECATED
		l.Warnf("DEPRECATED: Config 'build.log_level' is deprecated in v0.9, use 'log.level = \"%s\"' instead. Deprecated config will not break your functionality, its good to update to latest config.", logLevel)
	}

	return l
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
	return fmt.Sprintf("%s/%s.%s", importPrefix, name, inferVersionSeries())
}

func libDir(name string) string {
	importPath := libImportPath(name)
	return filepath.FromSlash(filepath.Join(gopath, "src", importPath))
}

func gitBranchName(dir string) string {
	if !ess.IsDir(dir) {
		cliLog.Tracef("Given path '%s' is not a directory", dir)
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

func goGet(pkgs ...string) error {
	for _, pkg := range pkgs {
		args := []string{"get", pkg}
		if _, err := execCmd(gocmd, args, false); err != nil {
			return err
		}
	}
	return nil
}

func waitForConnReady(port string) {
	port = ":" + port
	startTime := time.Now()
	for {
		if _, err := net.Dial("tcp", port); err != nil {
			if time.Since(startTime).Seconds() > (30 * time.Second).Seconds() {
				return
			}

			time.Sleep(10 * time.Millisecond)
			continue
		}
		return
	}
}

func installAahCLI() {
	verser := inferVersionSeries()
	args := []string{"install", fmt.Sprintf("%s/tools.%s/aah", importPrefix, verser)}
	if _, err := execCmd(gocmd, args, false); err != nil {
		logFatalf("Unable to compile CLI tool: %s", err)
	}
}

func fetchAahDeps() {
	// depList would have duplicate import paths
	// since we collect from more than one lib
	// TODO: improve efficiency
	var depList []string
	for _, i := range aahImportPaths() {
		depList = append(depList, libDependencyImports(i)...)
	}

	// infer not exists libraries on GOPATH using importpath
	notEixstsList := inferNotExistsDeps(depList)
	if len(notEixstsList) > 0 {
		if err := goGet(notEixstsList...); err != nil {
			logFatalf("Error during go get: %s", err)
		}
	}
}

func refreshCodebase(libDirs []string) {
	for _, ld := range libDirs {
		if err := gitPull(ld); err != nil {
			logFatalf("Unable to refresh library: %s", filepath.Base(ld))
		}
	}
}

func getAppImportPath(c *cli.Context) string {
	importPath := firstNonEmpty(c.String("i"), c.String("importpath"))
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		logFatalf("Given import path '%s' does not exists", importPath)
	}

	return importPath
}

func logFatal(v ...interface{}) {
	_ = log.SetPattern("%level %message")
	fatal(v...)
	_ = log.SetPattern(log.DefaultPattern)
}

func logFatalf(format string, v ...interface{}) {
	_ = log.SetPattern("%level %message")
	fatalf(format, v...)
	_ = log.SetPattern(log.DefaultPattern)
}

func logError(v ...interface{}) {
	cliLog.Error(append([]interface{}{"ERROR"}, v...))
}

func logErrorf(format string, v ...interface{}) {
	cliLog.Errorf("ERROR "+format, v...)
}

func stripGoPath(pkgFilePath string) string {
	idx := strings.Index(pkgFilePath, "src")
	return filepath.Clean(pkgFilePath[idx+4:])
}

func inferVersionSeries() string {
	verser := "v0"
	for _, d := range aahLibraryDirs() {
		baseName := filepath.Base(d)
		if strings.HasPrefix(baseName, "aah") {
			return strings.Split(baseName, ".")[1]
		}
	}
	return verser
}

func aahLibraryDirs() []string {
	dirs, err := ess.DirsPath(filepath.Join(gosrcDir, importPrefix), false)
	if err != nil {
		return []string{}
	}
	return dirs
}

func aahImportPaths() []string {
	var importPaths []string
	gsLen := len(gosrcDir)
	for _, d := range aahLibraryDirs() {
		p := d[gsLen+1:]
		if strings.Contains(p, "tools") {
			p += "/aah" // Note: this import path so always forward slash
		}
		importPaths = append(importPaths, p)
	}
	return importPaths
}

func libDependencyImports(importPath string) []string {
	var depList []string
	str, err := execCmd(gocmd, []string{"list", "-f", "{{.Imports}}", importPath}, false)
	if err != nil {
		logErrorf("Unable to infer dependency imports for %s", importPath)
		return []string{}
	}

	str = strings.TrimSpace(str)
	for _, i := range strings.Fields(str[1 : len(str)-1]) {
		depList = append(depList, strings.TrimSpace(i))
	}

	return depList
}

func inferNotExistsDeps(depList []string) []string {
	var notExistsList []string
	for _, d := range depList {
		if !ess.IsImportPathExists(d) && !ess.IsSliceContainsString(notExistsList, d) {
			notExistsList = append(notExistsList, d)
		}
	}
	return notExistsList
}
