// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"aahframework.org/aah"
	"aahframework.org/aah/router"
	"aahframework.org/essentials"
	"aahframework.org/log"
)

// buildApp method calls Go ast parser, generates main.go and builds aah
// application binary at Go bin directory
func buildApp() error {
	_ = log.SetPattern("%level:-5 %message")

	appBaseDir := aah.AppBaseDir()
	appCodeDir := filepath.Join(appBaseDir, "app")
	appImportPath := aah.AppImportPath()

	// clean up before we start build aah application
	ess.DeleteFiles(filepath.Join(appCodeDir, "main.go"))

	// excludes for Go AST processing
	excludes := ess.Excludes{
		"*_test.go",
		".*",
		"*.bak",
		"*.tmp",
		"vendor",
	}

	// Go AST processing for Controllers
	prg, errs := loadProgram(filepath.Join(appCodeDir, "controllers"), excludes)
	if len(errs) > 0 {
		errMsgs := []string{}
		for _, e := range errs {
			errMsgs = append(errMsgs, e.Error())
		}
		log.Fatal(strings.Join(errMsgs, "\n"))
	}

	// fetch all configured Controllers with action info
	prg.RouteMethods = router.AllControllerMethods()

	// call the process
	prg.Process()

	// Print router configuration missing/error details
	missingActions := []string{}
	for c, m := range prg.RouteMethods {
		for a, v := range m {
			if v == 1 && !router.IsDefaultAction(a) {
				missingActions = append(missingActions, fmt.Sprintf("%s.%s", c, a))
			}
		}
	}
	if len(missingActions) > 0 {
		log.Error("Following actions are configured in 'routes.conf', however not implemented in Controller:\n",
			strings.Join(missingActions, "\n"))
	}

	// get all the types info refered aah framework controller
	types := prg.FindTypeByEmbeddedType(fmt.Sprintf("%s.Controller", aahImportPath))
	configuredTypes := []*typeInfo{}
	for _, t := range types {
		if len(t.Methods) > 0 {
			configuredTypes = append(configuredTypes, t)
		}
	}

	importPaths := createImportPaths(configuredTypes)
	generateSource(appCodeDir, "main.go", aahMainTemplate, map[string]interface{}{
		"AppImportPath": appImportPath,
		"Controllers":   configuredTypes,
		"ImportPaths":   importPaths,
	})

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
          &aah.ParameterInfo{Name: "{{.Name}}", Type: reflect.TypeOf((*{{.Type.Name}})(nil)) },{{end}}
        },
      },
      {{end}}
    })
  {{end}}

  // aah.Start()
}
`
