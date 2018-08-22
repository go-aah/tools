// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// aahframework.org/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"aahframe.work/aah"
	"aahframe.work/aah/config"
	"aahframe.work/aah/essentials"
	"aahframe.work/aah/log"
	"gopkg.in/urfave/cli.v1"
)

func importPathRelwd() string {
	pwd, _ := os.Getwd() // #nosec

	var importPath string
	if idx := strings.Index(pwd, "src"); idx > 0 {
		srcDir := pwd[:idx+3]
		appDir := pwd
		for {
			if ess.IsFileExists(filepath.Join(appDir, aahProjectIdentifier)) {
				importPath, _ = filepath.Rel(srcDir, appDir)
				break
			}

			if appDir == srcDir {
				break
			}

			appDir = filepath.Dir(appDir)
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
	if !ess.IsFileExists(filepath.Join(appBaseDir, ".git")) {
		return version
	}

	gitArgs := []string{"-C", appBaseDir, "describe", "--always", "--dirty"}
	output, err := execCmd(gitcmd, gitArgs, false)
	if err != nil {
		return version
	}

	return strings.TrimSpace(output)
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
	cmd := exec.Command(cmdName, args...) // #nosec
	cliLog = initCLILogger(nil)
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
	lstn, err := net.Listen("tcp", ":0") // #nosec
	if err != nil {
		logError(err)
		return "0"
	}
	defer ess.CloseQuietly(lstn)

	return strconv.Itoa(lstn.Addr().(*net.TCPAddr).Port)
}

func initCLILogger(cfg *config.Config) *log.Logger {
	if cliLog != nil {
		return cliLog
	}
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

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Git methods
//___________________________________________________________________________

func gitCheckout(dir, branch string) error {
	if ess.IsFileExists(filepath.Join(dir, ".git")) {
		_, err := execCmd(gitcmd, []string{"-C", dir, "checkout", branch}, false)
		return err
	}
	return nil
}

func gitBranchName(dir string) string {
	if !ess.IsDir(dir) {
		cliLog.Tracef("Given path '%s' is not a directory", dir)
		return ""
	}

	if !ess.IsFileExists(filepath.Join(dir, ".git")) {
		return ""
	}

	gitArgs := []string{"-C", dir, "rev-parse", "--abbrev-ref", "HEAD"}
	output, _ := execCmd(gitcmd, gitArgs, false)
	return strings.TrimSpace(output)
}

func gitPull(dir string) error {
	if ess.IsFileExists(filepath.Join(dir, ".git")) {
		_, err := execCmd(gitcmd, []string{"-C", dir, "pull"}, false)
		return err
	}
	return nil
}

func checkoutBranch(aahLibDirs []string, branchName string) {
	var wg sync.WaitGroup
	for _, dir := range aahLibDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			baseName := filepath.Base(d)
			if err := gitCheckout(d, branchName); err != nil {
				logErrorf("Unable to switch library version, possibliy you may have local changes[%s]: %s", baseName, err)
			}
			cliLog.Tracef("Library '%s' have been switched to '%s' successfully", baseName, branchName)
		}(dir)
	}
	wg.Wait()
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// aah discovery and processing methods
//___________________________________________________________________________

func libImportPath(name string) string {
	return fmt.Sprintf("%s/%s.%s", importPrefix, name, inferVersionSeries())
}

func libDir(name string) string {
	importPath := libImportPath(name)
	return filepath.FromSlash(filepath.Join(gosrcDir, importPath))
}

func goGet(pkgs ...string) error {
	for _, pkg := range pkgs {
		if _, err := execCmd(gocmd, []string{"get", pkg}, false); err != nil {
			return err
		}
	}
	return nil
}

func installCLI() {
	if CliPackaged != "" {
		return
	}
	verser := inferVersionSeries()
	args := []string{"install", fmt.Sprintf("%s/tools.%s/aah", importPrefix, verser)}
	if _, err := execCmd(gocmd, args, false); err != nil {
		logFatalf("Unable to compile aah CLI: %s", err)
	}
}

func fetchLibDeps() {
	var notEixstsList []string
	var wg sync.WaitGroup
	for _, i := range aahImportPaths() {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if neList := inferNotExistsDeps(libDependencyImports(p)); len(neList) > 0 {
				notEixstsList = append(notEixstsList, neList...)
			}
		}(i)
	}
	wg.Wait()

	// infer not exists libraries on GOPATH using importpath
	if len(notEixstsList) > 0 {
		if err := goGet(notEixstsList...); err != nil {
			logFatalf("Error during go get: %s", err)
		}
	}
}

func refreshLibCode(libDirs []string) {
	var wg sync.WaitGroup
	for _, dir := range libDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			if err := gitPull(d); err != nil {
				logErrorf("Unable to refresh library, possibliy you may have local changes: %s", filepath.Base(d))
			}
		}(dir)
	}
	wg.Wait()
}

func stripGoSrcPath(pkgFilePath string) string {
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
	dirs, err := ess.DirsPathExcludes(filepath.Join(gosrcDir, importPrefix), false, ess.Excludes{"examples"})
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
	args := []string{"list", "-f", "{{.Imports}}", importPath}
	output, err := execCmd(gocmd, args, false)
	if err != nil {
		logErrorf("Unable to infer dependency imports for %s", importPath)
		return []string{}
	}

	pkgList := make(map[string]string)
	replacer := strings.NewReplacer("[", "", "]", "")
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if ln := replacer.Replace(strings.TrimSpace(scanner.Text())); ln != "" {
			for _, p := range strings.Fields(ln) {
				if p := strings.TrimSpace(p); p != "" {
					pkgList[p] = p
				}
			}
		}
	}

	var depList []string
	for _, p := range pkgList {
		depList = append(depList, p)
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

func readVersionNo(baseDir string) (string, error) {
	versionFile := filepath.Join(baseDir, "version.go")
	if !ess.IsFileExists(versionFile) {
		return "", errVersionNotExists
	}

	bytes, err := ioutil.ReadFile(versionFile)
	if err != nil {
		return "", err
	}

	result := verRegex.FindStringSubmatch(string(bytes))
	if len(result) >= 2 {
		return result[1], nil
	}

	return "Unknown", nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// other helper methods
//___________________________________________________________________________

func appImportPath(c *cli.Context) string {
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
	if cliLog == nil {
		_ = log.SetPattern("%level %message")
		fatal(v...)
		_ = log.SetPattern(log.DefaultPattern)
	} else {
		cliLog.Fatal(append([]interface{}{"FATAL"}, v...))
	}
}

func logFatalf(format string, v ...interface{}) {
	if cliLog == nil {
		_ = log.SetPattern("%level %message")
		fatalf(format, v...)
		_ = log.SetPattern(log.DefaultPattern)
	} else {
		cliLog.Fatalf("FATAL "+format, v...)
	}
}

func logError(v ...interface{}) {
	cliLog.Error(append([]interface{}{"ERROR"}, v...))
}

func logErrorf(format string, v ...interface{}) {
	cliLog.Errorf("ERROR "+format, v...)
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

func cleanupAutoGenFiles(appBaseDir string) {
	appMainGoFile := filepath.Join(appBaseDir, "app", "aah.go")
	appBuildDir := filepath.Join(appBaseDir, "build")
	cliLog.Debugf("Cleaning %s", appMainGoFile)
	cliLog.Debugf("Cleaning build directory %s", appBuildDir)
	ess.DeleteFiles(appMainGoFile, appBuildDir)
}

func cleanupAutoGenVFSFiles(appBaseDir string) {
	vfsFiles, _ := filepath.Glob(filepath.Join(appBaseDir, "app", "aah_*_vfs.go"))
	if len(vfsFiles) > 0 {
		cliLog.Debugf("Cleaning embed files %s", strings.Join(vfsFiles, "\n\t"))
		ess.DeleteFiles(vfsFiles...)
	}
}

func toLowerCamelCase(v string) string {
	var st []byte
	for idx := 0; idx < len(v); idx++ {
		c := v[idx]
		if c == '_' || c == ' ' {
			idx++
			st = append(st, []byte(strings.ToUpper(string(v[idx])))...)
		} else {
			st = append(st, c)
		}
	}
	return string(st)
}

func inferAppTmplBaseDir() string {
	aahBasePath := aahPath()
	baseDir := filepath.Join(aahBasePath, "app-templates", "generic")
	if !ess.IsFileExists(baseDir) {
		tmplRepo := "https://github.com/go-aah/app-templates.git"
		cliLog.Debugf("Downloading aah quick start app templates from %s", tmplRepo)
		gitArgs := []string{"clone", tmplRepo, filepath.Dir(baseDir)}
		if _, err := execCmd(gitcmd, gitArgs, false); err != nil {
			logErrorf("Unable to download aah app-template from %s", tmplRepo)
			return ""
		}
	}
	return baseDir
}

func aahPath() string {
	s := os.Getenv("AAHPATH")
	if s == "" {
		return filepath.Join(userHomeDir(), ".aah")
	}
	return s
}

func userHomeDir() string {
	if isWindowsOS() {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if ess.IsStrEmpty(home) {
			home = os.Getenv("USERPROFILE")
		}
		return filepath.Clean(home)
	}

	env := "HOME"
	if runtime.GOOS == "plan9" {
		env = "home"
	}
	if home := os.Getenv(env); home != "" {
		return filepath.Clean(home)
	}

	return ""
}

func goCmdName() string {
	if name := os.Getenv("AAHVGO"); name != "" {
		return "vgo"
	}
	return "go"
}

func fetchFile(dst, src string) error {
	resp, err := http.Get(src)
	if err != nil {
		return err
	}
	defer ess.CloseQuietly(resp.Body)

	_ = ess.MkDirAll(filepath.Dir(dst), permRWXRXRX)
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer ess.CloseQuietly(f)

	_, err = io.Copy(f, resp.Body)
	return err
}
