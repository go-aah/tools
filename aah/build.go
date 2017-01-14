// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

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

	appBaseDir := aah.AppBaseDir()
	appImportPath := aah.AppImportPath()
	appCodeDir := filepath.Join(appBaseDir, "app")
	appControllersPath := filepath.Join(appCodeDir, "controllers")

	// clean up before we start build aah application
	ess.DeleteFiles(filepath.Join(appCodeDir, "main.go"))

	// read build config from 'aah.project'
	aahProjectFile := filepath.Join(appBaseDir, "aah.project")

	log.Infof("Reading aah project file: %s", aahProjectFile)
	buildCfg, err := config.LoadFile(aahProjectFile)
	if err != nil {
		log.Fatalf("aah project file error: %s", err)
	}

	// excludes for Go AST processing
	excludes, _ := buildCfg.StringList("build.excludes")

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
	controllers := prg.FindTypeByEmbeddedType(fmt.Sprintf("%s.Controller", aahImportPath))
	importPaths := prg.CreateImportPaths(controllers)

	generateSource(appCodeDir, "main.go", aahMainTemplate, map[string]interface{}{
		"AppImportPath": appImportPath,
		"Controllers":   controllers,
		"ImportPaths":   importPaths,
	})

	if err := checkAndGetAppDeps(appImportPath, buildCfg); err != nil {
		log.Fatal(err)
	}

	// TODO further build implementation

	return nil
}

func generateSource(dir, filename, templateSource string, templateArgs map[string]interface{}) {
	if !ess.IsFileExists(dir) {
		if err := ess.MkDirAll(dir, 0644); err != nil {
			log.Fatal(err)
		}
	}

	file := filepath.Join(dir, filename)
	tmpl := template.Must(template.New("").Parse(templateSource))

	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, templateArgs); err != nil {
		log.Fatalf("Unable to render template: %s", err)
	}

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

func execCmd(cmdName string, args []string) (string, error) {
	cmd := exec.Command(cmdName, args...)
	log.Info("Exec: ", strings.Join(cmd.Args, " "))

	bytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Generate Templates
//___________________________________

const aahMainTemplate = `// aah framework - https://aahframework.org
// FILE: main.go
// GENERATED CODE - DO NOT EDIT

package main

import (
	"flag"
	"reflect"
	"aahframework.org/aah"{{range $k, $v := $.ImportPaths}}
	{{$v}} "{{$k}}"{{end}}
)

var (
	// So compiler won't complain if the generated code doesn't reference reflect package...
	// _ = reflect.Invalid
)

func main() {
  flag.Parse()

  aah.Init("{{.AppImportPath}}")

  // Adding all the controllers which refers 'aah.Controller' directly
  // or indirectly from app/controllers/** {{range $i, $c := .Controllers}}
  aah.AddController((*{{index $.ImportPaths .ImportPath}}.{{.Name}})(nil),
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
