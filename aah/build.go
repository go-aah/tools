// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"aahframework.org/aah"
	"aahframework.org/aah/router"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

// buildApp method calls Go ast parser, generates main.go and builds aah
// application binary at Go bin directory
func buildApp() error {
	_ = log.SetPattern("%level:-5 %message")

	// app variables
	appBaseDir := aah.AppBaseDir()
	appImportPath := aah.AppImportPath()
	appCodeDir := filepath.Join(appBaseDir, "app")
	appControllersPath := filepath.Join(appCodeDir, "controllers")

	// read build config from 'aah.project'
	aahProjectFile := filepath.Join(appBaseDir, "aah.project")

	log.Infof("Reading aah project file: %s", aahProjectFile)
	buildCfg, err := config.LoadFile(aahProjectFile)
	if err != nil {
		log.Fatalf("aah project file error: %s", err)
	}

	// excludes for Go AST processing
	excludes, _ := buildCfg.StringList("build.ast_excludes")

	// get all configured Controllers with action info
	registeredActions := router.RegisteredActions()

	// Go AST processing for Controllers
	prg, errs := loadProgram(appControllersPath, ess.Excludes(excludes), registeredActions)
	if len(errs) > 0 {
		errMsgs := []string{}
		for _, e := range errs {
			errMsgs = append(errMsgs, e.Error())
		}
		log.Fatal(strings.Join(errMsgs, "\n"))
	}

	// call the process
	prg.Process()

	// Print router configuration missing/error details
	missingActions := []string{}
	for c, m := range prg.RegisteredActions {
		for a, v := range m {
			if v == 1 && !router.IsDefaultAction(a) {
				missingActions = append(missingActions, fmt.Sprintf("%s.%s", c, a))
			}
		}
	}
	if len(missingActions) > 0 {
		log.Error("Following actions are configured in 'routes.conf', however not implemented in Controller:\n\t",
			strings.Join(missingActions, "\n\t"))
	}

	// get all the types info refered aah framework controller
	appControllers := prg.FindTypeByEmbeddedType(fmt.Sprintf("%s.Controller", aahImportPath))
	appImportPaths := prg.CreateImportPaths(appControllers)

	// prepare aah application version and build date
	appVersion := getAppVersion(appBaseDir, buildCfg)
	appBuildDate := getBuildDate()

	// create go build arguments
	buildArgs := []string{"build"}

	flags, _ := buildCfg.StringList("build.flags")
	buildArgs = append(buildArgs, flags...)

	if ldflags := buildCfg.StringDefault("build.ldflags", ""); !ess.IsStrEmpty(ldflags) {
		buildArgs = append(buildArgs, "-ldflags", ldflags)
	}

	if tags := buildCfg.StringDefault("build.tags", ""); !ess.IsStrEmpty(tags) {
		buildArgs = append(buildArgs, "-tags", tags)
	}

	// binary name creation
	name := strings.Replace(buildCfg.StringDefault("name", aah.AppName()), " ", "_", -1)
	appBinaryName := buildCfg.StringDefault("build.binary_name", name)
	if isWindows {
		appBinaryName += ".exe"
	}

	appBinary := filepath.Join(gopath, "bin", "aah.d", appImportPath, appBinaryName)
	buildArgs = append(buildArgs, "-o", appBinary)

	// main.go location e.g. path/to/import/app
	buildArgs = append(buildArgs, path.Join(appImportPath, "app"))

	// clean previous main.go and binary file up before we start the build
	appMainGoFile := filepath.Join(appCodeDir, "main.go")
	log.Infof("Cleaning %s", appMainGoFile)
	log.Infof("Cleaning %s", appBinary)
	ess.DeleteFiles(appMainGoFile, appBinary)

	generateSource(appCodeDir, "main.go", aahMainTemplate, map[string]interface{}{
		"AahVersion":     aah.Version,
		"AppImportPath":  appImportPath,
		"AppVersion":     appVersion,
		"AppBuildDate":   appBuildDate,
		"AppBinaryName":  appBinaryName,
		"AppControllers": appControllers,
		"AppImportPaths": appImportPaths,
	})

	if err = checkAndGetAppDeps(appImportPath, buildCfg); err != nil {
		log.Fatal(err)
	}

	// execute aah applictaion build
	if _, err = execCmd(gocmd, buildArgs); err != nil {
		log.Fatal(err)
	}

	log.Infof("'%s' application build successful.", aah.AppName())

	return nil
}

func generateSource(dir, filename, templateSource string, templateArgs map[string]interface{}) {
	if !ess.IsFileExists(dir) {
		if err := ess.MkDirAll(dir, 0644); err != nil {
			log.Fatal(err)
		}
	}

	file := filepath.Join(dir, filename)
	buf := &bytes.Buffer{}
	renderTmpl(buf, templateSource, templateArgs)

	if err := ioutil.WriteFile(file, buf.Bytes(), 0755); err != nil {
		log.Fatalf("aah '%s' file write error: %s", filename, err)
	}
}

// checkAndGetAppDeps method project dependencies is present otherwise
// it tries to get it if any issues it will return error. It internally uses
// go list command.
// 		go list -f '{{ join .Imports "\n" }}' aah-app/import/path
//
func checkAndGetAppDeps(appImportPath string, cfg *config.Config) error {
	args := []string{"list", "-f", "{{.Imports}}", appImportPath}

	output, err := execCmd(gocmd, args)
	if err != nil {
		log.Errorf("unable to get application dependencies: %s", err)
		return nil
	}

	output = strings.Replace(strings.Replace(output, "]", "", -1), "[", "", -1)
	output = strings.Replace(strings.Replace(output, "\r", "", -1), "\n", "", -1)
	if ess.IsStrEmpty(output) {
		// all dependencies is available
		return nil
	}

	notExistsPkgs := []string{}
	for _, pkg := range strings.Split(output, " ") {
		if !ess.IsImportPathExists(pkg) {
			notExistsPkgs = append(notExistsPkgs, pkg)
		}
	}

	if cfg.BoolDefault("build.go_get", true) && len(notExistsPkgs) > 0 {
		log.Info("Getting application dependencies ...")
		for _, pkg := range notExistsPkgs {
			args := []string{"get", pkg}
			if _, err := execCmd(gocmd, args); err != nil {
				return err
			}
		}
	}

	return nil
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
		output, err := execCmd(gitcmd, gitArgs)
		if err != nil {
			fmt.Println(err)
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

func execCmd(cmdName string, args []string) (string, error) {
	cmd := exec.Command(cmdName, args...)
	log.Info("Executing ", strings.Join(cmd.Args, " "))

	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Generate Templates
//___________________________________

const aahMainTemplate = `// aah framework v{{.AahVersion}} - https://aahframework.org
// FILE: main.go
// GENERATED CODE - DO NOT EDIT

package main

import (
	"flag"
	"fmt"
	"reflect"

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"{{range $k, $v := $.AppImportPaths}}
	{{$v}} "{{$k}}"{{end}}
)

var (
	appBinaryName = "{{.AppBinaryName}}"
	appVersion = "{{.AppVersion}}"
	appBuildDate = "{{.AppBuildDate}}"
)

func main() {
	// Defining flags
	version := flag.Bool("version", false, "Display application version and build date.")
	configPath := flag.String("config", "", "Absolute path of external config file.")
	flag.Parse()

	// display application information
	if *version {
		fmt.Printf("%-12s: %s\n", "Binary Name", appBinaryName)
		fmt.Printf("%-12s: %s\n", "Version", appVersion)
		fmt.Printf("%-12s: %s\n", "Build Date", appBuildDate)
		return
	}

  aah.Init("{{.AppImportPath}}")

	// Loading externally supplied config file
	if !ess.IsStrEmpty(*configPath) {
		externalConfig, err := config.LoadFile(*configPath)
		if err != nil {
			log.Fatalf("Unable to load external config: %s", *configPath)
		}

		aah.MergeAppConfig(externalConfig)
	}

  // Adding all the controllers which refers 'aah.Controller' directly
  // or indirectly from app/controllers/** {{range $i, $c := .AppControllers}}
  aah.AddController((*{{index $.AppImportPaths .ImportPath}}.{{.Name}})(nil),
    []*aah.MethodInfo{
      {{range .Methods}}&aah.MethodInfo{
        Name: "{{.Name}}",
        Parameters: []*aah.ParameterInfo{ {{range .Parameters}}
          &aah.ParameterInfo{Name: "{{.Name}}", Type: reflect.TypeOf((*{{.Type.Name}})(nil))},{{end}}
        },
      },
      {{end}}
    })
  {{end}}

  // aah.Start()
}
`
