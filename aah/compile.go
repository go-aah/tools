// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
	"aahframework.org/router.v0"
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

// compileApp method calls Go ast parser, generates main.go and builds aah
// application binary at Go bin directory
func compileApp(buildCfg *config.Config) (string, error) {
	// app variables
	appBaseDir := aah.AppBaseDir()
	appImportPath := aah.AppImportPath()
	appCodeDir := filepath.Join(appBaseDir, "app")
	appControllersPath := filepath.Join(appCodeDir, "controllers")
	appBuildDir := filepath.Join(appBaseDir, "build")

	appName := buildCfg.StringDefault("name", aah.AppName())
	log.Infof("Compile starts for '%s' [%s]", appName, appImportPath)

	// excludes for Go AST processing
	excludes, _ := buildCfg.StringList("build.ast_excludes")

	// get all configured Controllers with action info
	registeredActions := aah.AppRouter().RegisteredActions()

	// Go AST processing for Controllers
	prg, errs := loadProgram(appControllersPath, ess.Excludes(excludes), registeredActions)
	if len(errs) > 0 {
		errMsgs := []string{}
		for _, e := range errs {
			errMsgs = append(errMsgs, e.Error())
		}
		return "", errors.New(strings.Join(errMsgs, "\n"))
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

	// get all the types info referred aah framework context embedded
	appControllers := prg.FindTypeByEmbeddedType(fmt.Sprintf("%s.Context", aahImportPath))
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

	appBinary := appBinaryFile(buildCfg, appBuildDir)
	appBinaryName := filepath.Base(appBinary)
	buildArgs = append(buildArgs, "-o", appBinary)

	// main.go location e.g. path/to/import/app
	buildArgs = append(buildArgs, path.Join(appImportPath, "app"))

	// clean previous main.go and binary file up before we start the build
	appMainGoFile := filepath.Join(appCodeDir, "aah.go")
	log.Debugf("Cleaning %s", appMainGoFile)
	log.Debugf("Cleaning build directory %s", appBuildDir)
	ess.DeleteFiles(appMainGoFile, appBuildDir)

	generateSource(appCodeDir, "aah.go", aahMainTemplate, map[string]interface{}{
		"AahVersion":     aah.Version,
		"AppImportPath":  appImportPath,
		"AppVersion":     appVersion,
		"AppBuildDate":   appBuildDate,
		"AppBinaryName":  appBinaryName,
		"AppControllers": appControllers,
		"AppImportPaths": appImportPaths,
	})

	// getting project dependencies if not exists in $GOPATH
	if err := checkAndGetAppDeps(appImportPath, buildCfg); err != nil {
		return "", fmt.Errorf("unable to get application dependencies: %s", err)
	}

	// execute aah applictaion build
	if _, err := execCmd(gocmd, buildArgs, false); err != nil {
		return "", err
	}

	log.Infof("Compile successful for '%s' [%s]", appName, appImportPath)

	return appBinary, nil
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

	if err := ioutil.WriteFile(file, buf.Bytes(), permRWXRXRX); err != nil {
		log.Fatalf("aah '%s' file write error: %s", filename, err)
	}
}

// checkAndGetAppDeps method project dependencies is present otherwise
// it tries to get it if any issues it will return error. It internally uses
// go list command.
// 		go list -f '{{ join .Imports "\n" }}' aah-app/import/path/app/...
//
func checkAndGetAppDeps(appImportPath string, cfg *config.Config) error {
	importPath := path.Join(appImportPath, "app", "...")
	args := []string{"list", "-f", "{{.Imports}}", importPath}

	output, err := execCmd(gocmd, args, false)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(output), "\r\n")
	for _, line := range lines {
		line = strings.Replace(strings.Replace(line, "]", "", -1), "[", "", -1)
		line = strings.Replace(strings.Replace(line, "\r", " ", -1), "\n", " ", -1)
		if ess.IsStrEmpty(line) {
			// all dependencies is available
			return nil
		}

		notExistsPkgs := []string{}
		for _, pkg := range strings.Split(line, " ") {
			if ess.IsStrEmpty(pkg) || ess.IsImportPathExists(pkg) {
				continue
			}
			notExistsPkgs = append(notExistsPkgs, pkg)
		}

		if cfg.BoolDefault("build.go_get", true) && len(notExistsPkgs) > 0 {
			log.Info("Getting application dependencies ...")
			for _, pkg := range notExistsPkgs {
				args := []string{"get", pkg}
				if _, err := execCmd(gocmd, args, false); err != nil {
					return err
				}
			}
		} else if len(notExistsPkgs) > 0 {
			log.Error("Below application dependencies are not exists, " +
				"enable 'build.go_get=true' in 'aah.project' for auto fetch")
			log.Fatal("\n", strings.Join(notExistsPkgs, "\n"))
		}
	}

	return nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Generate Templates
//___________________________________

const aahMainTemplate = `// GENERATED CODE - DO NOT EDIT
//
// aah framework v{{.AahVersion}} - https://aahframework.org
// FILE: aah.go
// DESC: aah application entry point

package main

import (
	"flag"
	"fmt"
	"reflect"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"{{ range $k, $v := $.AppImportPaths }}
	{{$v}} "{{$k}}"{{ end }}
)

var _ = reflect.Invalid

func main() {
	// Defining flags
	version := flag.Bool("version", false, "Display application name, version and build date.")
	configPath := flag.String("config", "", "Absolute path of external config file.")
	profile := flag.String("profile", "", "Environment profile name to activate. e.g: dev, qa, prod.")
	flag.Parse()

	aah.SetAppBuildInfo(&aah.BuildInfo{
		BinaryName: "{{.AppBinaryName}}",
		Version:    "{{.AppVersion}}",
		Date:       "{{.AppBuildDate}}",
	})

	// display application information
	if *version {
		fmt.Printf("%-12s: %s\n", "Binary Name", aah.AppBuildInfo().BinaryName)
		fmt.Printf("%-12s: %s\n", "Version", aah.AppBuildInfo().Version)
		fmt.Printf("%-12s: %s\n", "Build Date", aah.AppBuildInfo().Date)
		return
	}

	// Apply supplied external config file
	if !ess.IsStrEmpty(*configPath) {
		aah.OnInit(func(e *aah.Event) {
			externalConfig, err := config.LoadFile(*configPath)
			if err != nil {
				log.Fatalf("Unable to load external config: %s", *configPath)
			}

			log.Debug("Merging external config into aah application config")
			if err := aah.AppConfig().Merge(externalConfig); err != nil {
				log.Errorf("Unable to merge external config into aah application[%s]: %s", aah.AppName(), err)
			}
		})
	}

	// Apply environment profile
	if !ess.IsStrEmpty(*profile) {
	  aah.OnInit(func(e *aah.Event) {
	    aah.AppConfig().SetString("env.active", *profile)
	  })
	}

	aah.Init("{{.AppImportPath}}")

	// Adding all the controllers which refers 'aah.Context' directly
	// or indirectly from app/controllers/** {{ range $i, $c := .AppControllers }}
	aah.AddController((*{{index $.AppImportPaths .ImportPath}}.{{.Name}})(nil),
	  []*aah.MethodInfo{
	    {{ range .Methods }}&aah.MethodInfo{
	      Name: "{{.Name}}",
	      Parameters: []*aah.ParameterInfo{ {{ range .Parameters }}
	        &aah.ParameterInfo{Name: "{{.Name}}", Type: reflect.TypeOf((*{{.Type.Name}})(nil))},{{ end }}
	      },
	    },
	    {{ end }}
	  })
	{{ end }}

  aah.Start()
}
`
