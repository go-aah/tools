// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/build"
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
	"text/template"
	"time"

	"aahframe.work"
	"aahframe.work/config"
	"aahframe.work/console"
	"aahframe.work/essentials"
	"aahframe.work/log"
)

func goVersion() string {
	ver, err := execCmd(gocmd, []string{"version"}, false)
	if err != nil {
		logFatalf("Unable to infer go version: %v", err)
	}
	return strings.TrimPrefix(strings.Fields(ver)[2], "go")
}

func inferGo111AndAbove() bool {
	ver := strings.Join(strings.Split(goVersion(), ".")[:2], ".")
	verNum, err := strconv.ParseFloat(ver, 64)
	if err != nil {
		return false
	}
	return verNum >= float64(1.11)
}

func inferInsideGopath(dir string) bool {
	for _, gp := range filepath.SplitList(build.Default.GOPATH) {
		if strings.HasPrefix(dir, gp) {
			return true
		}
	}
	return false
}

func appImportPath(c *console.Context) string {
	// get import path from go.mod
	if ess.IsFileExists(goModIdentifier) {
		output, err := execCmd(gocmd, []string{"list", "-m", "-json"}, false)
		if err == nil {
			mods := parseGoListModJSON(output)
			if len(mods) > 0 {
				return mods[0].Path
			}
		}
	}

	var importPath string
	pwd, _ := os.Getwd() // #nosec
	if i := strings.Index(pwd, "src"); i > 0 {
		srcDir, appDir := pwd[:i+3], pwd
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

	if ess.IsStrEmpty(importPath) && ess.IsFileExists(aahProjectIdentifier) {
		pwd, _ := os.Getwd() // #nosec
		return filepath.Base(pwd)
	}

	return filepath.ToSlash(importPath)
}

func parseGoListModJSON(rawCmdJSON string) []*module {
	if ess.IsStrEmpty(rawCmdJSON) {
		return nil
	}
	rawCmdJSON = strings.Replace(strings.Replace(strings.Replace(rawCmdJSON, "\n", "", -1), "\t", "", -1), "}{", "}\n{", -1)
	scanner := bufio.NewScanner(strings.NewReader(rawCmdJSON))
	var mods []*module
	for scanner.Scan() {
		m := &module{}
		if err := json.Unmarshal([]byte(scanner.Text()), m); err != nil {
			continue
		}
		mods = append(mods, m)
	}
	return mods
}

func chdirIfRequired(importPath string) {
	if p := aahInventory.Lookup(importPath); p != nil {
		if cwd, err := os.Getwd(); err == nil {
			if !ess.IsFileExists(aahProjectIdentifier) && !strings.EqualFold(cwd, p.Dir) {
				if err = os.Chdir(p.Dir); err != nil {
					logError(err)
				}
			}
		}
	}
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
		v = strings.TrimSpace(v)
		if len(v) > 0 {
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
// version from compiled binary.
//
// Application build date value priority are -
// 		1. Env variable - AAH_APP_BUILD_TIMESTAMP
// 		2. Env variable - AAH_APP_BUILD_DATE (deprecated in v0.12.0, highly recommended to use timestamp)
// 		3. Created with time.Now().Format(time.RFC3339)
func getBuildTimestamp() string {
	// From env variable
	if buildTimestamp := os.Getenv("AAH_APP_BUILD_TIMESTAMP"); !ess.IsStrEmpty(buildTimestamp) {
		return buildTimestamp
	}
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
	replacer := strings.NewReplacer(" ", "_", ".", "_")
	appBinaryName := buildCfg.StringDefault("build.binary_name", replacer.Replace(aah.App().Name()))
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

func isAahProject(dir ...string) bool {
	if len(dir) == 0 {
		return ess.IsFileExists(aahProjectIdentifier)
	}
	return strings.HasSuffix(dir[0], aahProjectIdentifier) && ess.IsFileExists(dir[0])
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
	if cfg == nil && cliLog != nil {
		return cliLog
	}
	if cfg == nil {
		cfg = config.NewEmpty()
	}

	printDeprecateInfo := false
	logLevel := cfg.StringDefault("log.level", "info")
	if level, found := cfg.String("build.log_level"); found {
		logLevel = level
		printDeprecateInfo = true
	}

	logCfg := config.NewEmpty()
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

func gitPull(dir string) error {
	if ess.IsFileExists(filepath.Join(dir, ".git")) {
		_, err := execCmd(gitcmd, []string{"-C", dir, "pull", "--all"}, false)
		return err
	}
	return nil
}

func gitCheckout(dir, branch string) error {
	if ess.IsFileExists(filepath.Join(dir, ".git")) {
		_, err := execCmd(gitcmd, []string{"-C", dir, "checkout", branch}, false)
		return err
	}
	return nil
}

func goGet(pkgs ...string) error {
	for _, pkg := range pkgs {
		if _, err := execCmd(gocmd, []string{"get", pkg}, false); err != nil {
			return err
		}
	}
	return nil
}

func stripGoSrcPath(pkgFilePath string) string {
	idx := strings.Index(pkgFilePath, "src")
	return filepath.Clean(pkgFilePath[idx+4:])
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
				if p = strings.TrimSpace(p); p != "" {
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

	return "unknown", nil
}

func logFatal(v ...interface{}) {
	cliLog.Fatal(append([]interface{}{"FATAL "}, v...)...)
}

func logFatalf(format string, v ...interface{}) {
	cliLog.Fatalf("FATAL "+format, v...)
}

func logError(v ...interface{}) {
	cliLog.Error(append([]interface{}{"ERROR "}, v...)...)
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
