// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"aahframe.work"
	"aahframe.work/ainsp"
	"aahframe.work/config"
	"aahframe.work/essentials"
	"aahframe.work/router"
)

type compileArgs struct {
	Cmd        string
	ProxyPort  string
	ProjectCfg *config.Config
	AppPack    bool
	AppEmbed   bool
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

// compileApp method calls Go ast parser, generates main.go and builds aah
// application binary at Go bin directory
func compileApp(args *compileArgs) (string, error) {
	projectCfg := args.ProjectCfg

	// app variables
	app := aah.App()
	appBaseDir := app.BaseDir()
	appImportPath := app.ImportPath()
	appCodeDir := filepath.Join(appBaseDir, "app")
	appControllersPath := filepath.Join(appCodeDir, "controllers")
	appWebSocketsPath := filepath.Join(appCodeDir, "websockets")
	appBuildDir := filepath.Join(appBaseDir, "build")

	appName := projectCfg.StringDefault("name", app.Name())
	cliLog.Infof("Compile starts for '%s' [%s]", appName, appImportPath)

	// excludes for Go AST processing
	excludes, _ := projectCfg.StringList("build.ast_excludes")

	// get all configured Controllers with action info
	registeredActions := app.Router().RegisteredActions()

	// Go AST processing for Controllers
	acntlr, errs := ainsp.Inspect(appControllersPath, appImportPath, ess.Excludes(excludes), registeredActions)
	if len(acntlr.Packages) > 0 {
		if len(errs) > 0 {
			errMsgs := []string{}
			for _, e := range errs {
				errMsgs = append(errMsgs, e.Error())
			}
			return "", errors.New(strings.Join(errMsgs, "\n"))
		}

		// Print router configuration missing/error details
		missingActions := []string{}
		for c, m := range acntlr.RegisteredActions {
			for a, v := range m {
				if v == 1 && !router.IsDefaultAction(a) {
					missingActions = append(missingActions, fmt.Sprintf("%s.%s", c, a))
				}
			}
		}
		if len(missingActions) > 0 {
			logError("Following actions are configured in 'routes.conf', however not implemented in Controller:\n\t",
				strings.Join(missingActions, "\n\t"))
		}
	}

	appImportPaths := map[string]string{}

	// get all the types info referred aah framework context embedded
	appControllers := acntlr.FindTypeByEmbeddedType(aahImportPath + ".Context")
	appImportPaths = acntlr.CreateImportPaths(appControllers, appImportPaths)
	appSecurity := appSecurity(app.Config(), appImportPaths)

	// Go AST processing for WebSockets
	registeredWSActions := app.Router().RegisteredWSActions()
	wsc, errs := ainsp.Inspect(appWebSocketsPath, appImportPath, ess.Excludes(excludes), registeredWSActions)
	if len(wsc.Packages) > 0 {
		if len(errs) > 0 {
			errMsgs := []string{}
			for _, e := range errs {
				errMsgs = append(errMsgs, e.Error())
			}
			return "", errors.New(strings.Join(errMsgs, "\n"))
		}

		// Print router configuration missing/error details
		missingWSActions := []string{}
		for c, m := range wsc.RegisteredActions {
			for a, v := range m {
				if v == 1 && !router.IsDefaultAction(a) {
					missingWSActions = append(missingWSActions, fmt.Sprintf("%s.%s", c, a))
				}
			}
		}
		if len(missingWSActions) > 0 {
			logError("Following WebSocket actions are configured in 'routes.conf', however not implemented in WebSocket:\n\t",
				strings.Join(missingWSActions, "\n\t"))
		}
	}

	appWebSockets := wsc.FindTypeByEmbeddedType(aahImportPath + "/ws.Context")
	appImportPaths = wsc.CreateImportPaths(appWebSockets, appImportPaths)

	if len(appControllers) == 0 && len(appWebSockets) == 0 {
		return "", fmt.Errorf("It seems your application have zero controller or websocket")
	}

	if len(appControllers) > 0 || len(appWebSockets) > 0 {
		appImportPaths[aahImportPath+"/ainsp"] = "ainsp"
	}

	// prepare aah application version and build date
	appVersion := getAppVersion(appBaseDir, projectCfg)
	appBuildTimestamp := getBuildTimestamp()

	// create go build arguments
	buildArgs := []string{"build"}

	if flags, found := projectCfg.StringList("build.flags"); found {
		buildArgs = append(buildArgs, flags...)
	}

	if ldflags := projectCfg.StringDefault("build.ldflags", ""); !ess.IsStrEmpty(ldflags) {
		buildArgs = append(buildArgs, "-ldflags", ldflags)
	}

	if tags := projectCfg.StringDefault("build.tags", ""); !ess.IsStrEmpty(tags) {
		buildArgs = append(buildArgs, "-tags", tags)
	}

	appBinary := appBinaryFile(projectCfg, appBuildDir)
	appBinaryName := filepath.Base(appBinary)
	buildArgs = append(buildArgs, "-o", appBinary)

	// main.go location e.g. path/to/import/app
	buildArgs = append(buildArgs, path.Join(appImportPath, "app"))

	if err := generateSource(filepath.Join(appBaseDir, "app", "generated"), "add_controllers.go",
		aahControllerTemplate, map[string]interface{}{
			"AahVersion":        strings.TrimPrefix(strings.TrimSpace(aahVer), "v"),
			"AppImportPath":     appImportPath,
			"AppVersion":        appVersion,
			"AppBuildTimestamp": appBuildTimestamp,
			"AppBuildGoVersion": goVersion(),
			"AppBinaryName":     appBinaryName,
			"AppControllers":    appControllers,
			"AppWebSockets":     appWebSockets,
			"AppImportPaths":    appImportPaths,
			"AppSecurity":       appSecurity,
			"AppIsPackaged":     args.AppPack,
			"AppIsEmbedded":     args.AppEmbed,
		}); err != nil {
		return "", err
	}

	if err := generateSource(filepath.Join(appBaseDir, "app"), "aah.go", aahMainTemplate,
		map[string]interface{}{
			"AahVersion":    strings.TrimPrefix(strings.TrimSpace(aahVer), "v"),
			"AppImportPath": appImportPath,
		}); err != nil {
		return "", err
	}

	// getting project dependencies if not exists in $GOPATH
	if err := checkAndGetAppDeps(appImportPath, projectCfg); err != nil {
		return "", fmt.Errorf("unable to get application dependencies: %s", err)
	}

	// execute aah applictaion build
	if _, err := execCmd(gocmd, buildArgs, false); err != nil {
		return "", err
	}

	cliLog.Infof("Compile successful for '%s' [%s]", appName, appImportPath)

	return appBinary, nil
}

func generateSource(dir, filename, templateSource string, templateArgs map[string]interface{}) error {
	if !ess.IsFileExists(dir) {
		if err := ess.MkDirAll(dir, permRWXRXRX); err != nil {
			return err
		}
	}

	file := filepath.Join(dir, filename)
	buf := &bytes.Buffer{}
	err := renderTmpl(buf, templateSource, templateArgs)
	if err != nil {
		return err
	}

	b := buf.Bytes()
	if strings.HasSuffix(filename, ".go") {
		if b, err = format.Source(b); err != nil {
			return fmt.Errorf("aah '%s' file format source error: %s", filename, err)
		}
	}

	if err := ioutil.WriteFile(file, b, permRWXRXRX); err != nil {
		return fmt.Errorf("aah '%s' file write error: %s", filename, err)
	}
	return nil
}

var notExistRegex = regexp.MustCompile(`cannot find package "(.*)" in any of`)

// checkAndGetAppDeps method project dependencies is present otherwise
// it tries to get it if any issues it will return error. It internally uses
// go list command.
// 		go list -f '{{ join .Imports "\n" }}' aah-app/import/path/app/...
//
func checkAndGetAppDeps(appImportPath string, cfg *config.Config) error {
	if ess.IsFileExists(goModIdentifier) || !strings.HasPrefix(aah.App().BaseDir(), gopath) {
		return nil
	}
	debList := libDependencyImports(path.Join(appImportPath, "app", "..."))
	if len(debList) == 0 {
		return nil
	}

	args := append([]string{"list"}, debList...)
	b, _ := exec.Command(gocmd, args...).CombinedOutput() // #nosec
	notExistsPkgs := []string{}
	matches := notExistRegex.FindAllStringSubmatch(string(b), -1)
	for _, m := range matches {
		notExistsPkgs = append(notExistsPkgs, m[1])
	}

	if cfg.BoolDefault("build.dep_get", true) && len(notExistsPkgs) > 0 {
		cliLog.Infof("Getting application dependencies ...\n---> %s",
			strings.Join(notExistsPkgs, "\n---> "))
		if err := goGet(notExistsPkgs...); err != nil {
			return err
		}
	} else if len(notExistsPkgs) > 0 {
		return fmt.Errorf("Below application dependencies does not exist, "+
			"enable 'build.dep_get=true' in 'aah.project' for auto fetch\n---> %s",
			strings.Join(notExistsPkgs, "\n---> "))
	}

	return nil
}

func appSecurity(appCfg *config.Config, appImportPaths map[string]string) map[string]interface{} {
	securityInfo := make(map[string]interface{})
	importPathPrefix := path.Join(aah.App().ImportPath(), "app")
	keyPrefixAuthScheme := "security.auth_schemes"

	for _, keyAuthScheme := range appCfg.KeysByPath(keyPrefixAuthScheme) {
		keyPrefixAuthSchemeCfg := keyPrefixAuthScheme + "." + keyAuthScheme

		// Basic auth - file realm check
		if appCfg.StringDefault(keyPrefixAuthSchemeCfg+".scheme", "") == "basic" {
			fileRealmPath := appCfg.StringDefault(keyPrefixAuthSchemeCfg+".file_realm", "")
			if !ess.IsStrEmpty(fileRealmPath) {
				continue
			}
		}

		isAuthSchemeCfg := false
		authSchemeInfo := struct {
			Authenticator string
			Principal     string
			Authorizer    string
		}{}

		// Authenticator
		authenticator := appCfg.StringDefault(keyPrefixAuthSchemeCfg+".authenticator", "")
		if !ess.IsStrEmpty(authenticator) {
			authSchemeInfo.Authenticator = prepareAuthAlias(
				keyAuthScheme+"sec", authenticator, importPathPrefix, appImportPaths)
			isAuthSchemeCfg = true
		}

		// Principal Provider
		principal := appCfg.StringDefault(keyPrefixAuthSchemeCfg+".principal", "")
		if !ess.IsStrEmpty(principal) {
			authSchemeInfo.Principal = prepareAuthAlias(
				keyAuthScheme+"sec", principal, importPathPrefix, appImportPaths)
			isAuthSchemeCfg = true
		}

		// Authorizer
		authorizer := appCfg.StringDefault(keyPrefixAuthSchemeCfg+".authorizer", "")
		if !ess.IsStrEmpty(authorizer) {
			authSchemeInfo.Authorizer = prepareAuthAlias(
				keyAuthScheme+"secz", authorizer, importPathPrefix, appImportPaths)
			isAuthSchemeCfg = true
		}

		if isAuthSchemeCfg {
			securityInfo[keyAuthScheme] = authSchemeInfo
		}
	}

	if len(securityInfo) == 0 {
		return nil
	}

	return securityInfo
}

func prepareAuthAlias(keyAuthAlias, auth, importPathPrefix string, appImportPaths map[string]string) string {
	var authAlias string
	importPath := path.Dir(auth)
	if strings.HasPrefix(auth, "security") {
		importPath = path.Join(importPathPrefix, importPath)
	}

	if alias, found := appImportPaths[importPath]; found {
		authAlias = alias
	} else {
		authAlias = keyAuthAlias
		appImportPaths[importPath] = authAlias
	}
	return authAlias + "." + path.Base(auth)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Generate Templates
//___________________________________

const aahControllerTemplate = `// Code generated by aah CLI, DO NOT EDIT
//
// aah framework v{{ .AahVersion }} - https://aahframework.org
// FILE: add_controllers.go
// DESC: aah application controllers, websockets registration

package generated

import (
	"reflect"

	"aahframe.work"{{ if .AppSecurity }}
	"aahframe.work/security/authc"
	"aahframe.work/security/authz"{{ end }}{{ range $k, $v := $.AppImportPaths }}
	{{ $v }} "{{ $k }}"{{ end }}
)

var _ = reflect.Invalid

func init() {
	app := aah.App()
	app.SetBuildInfo(&aah.BuildInfo{
		BinaryName: "{{ .AppBinaryName }}",
		Version:    "{{ .AppVersion }}",
		Timestamp:  "{{ .AppBuildTimestamp }}",
		AahVersion: "{{ .AahVersion }}",
		GoVersion:  "{{ .AppBuildGoVersion }}",
	})
	{{ if .AppIsPackaged }}app.SetPackaged({{ .AppIsPackaged }}){{ end }}

	{{ if gt (len .AppControllers) 0 -}}
	// Adding all the application controllers which refers 'aah.Context' directly
	// or indirectly from app/controllers/** {{ range $i, $c := .AppControllers }}
	app.AddController((*{{ index $.AppImportPaths .ImportPath }}.{{ .Name }})(nil), []*ainsp.Method{ {{ range .Methods }}
		{Name: "{{ .Name }}"{{ if gt (len .Parameters) 0 }}, Parameters: []*ainsp.Parameter{ {{ range .Parameters }}
	    {Name: "{{ .Name }}", Type: reflect.TypeOf((*{{ .Type.Name }})(nil))},{{- end }}
		}{{ end }}},{{ end }}
	}){{- end }}
	{{ end -}}

	{{ if gt (len .AppWebSockets) 0 -}}
	// Adding all the application websockets which refers 'ws.Context' directly
	// or indirectly from app/websockets/** {{ range $i, $c := .AppWebSockets }}
	app.AddWebSocket((*{{ index $.AppImportPaths .ImportPath }}.{{ .Name }})(nil), []*ainsp.Method{ {{ range .Methods }}
		{Name: "{{ .Name }}"{{ if gt (len .Parameters) 0 }}, Parameters: []*ainsp.Parameter{ {{ range .Parameters }}
	    {Name: "{{ .Name }}", Type: reflect.TypeOf((*{{ .Type.Name }})(nil))},{{- end }}
	  }{{ end }}},{{ end }}
	}){{- end }}
	{{ end -}}

	{{ if .AppSecurity }}
	type setprincipal interface {
		SetPrincipalProvider(principal authc.PrincipalProvider) error
	}
	type setauthenticator interface {
		SetAuthenticator(authenticator authc.Authenticator) error
	}
	type setauthorizer interface {
		SetAuthorizer(authorizer authz.Authorizer) error
	}

	// Initialize application security auth schemes - Authenticator,
	// PrincipalProvider & Authorizer
	secMgr := app.SecurityManager()
	{{- range $k, $v := $.AppSecurity }}{{ $vPrefix := (variablename $k)  }}
	{{ $vPrefix }}AuthScheme := secMgr.AuthScheme("{{ $k }}")
	{{ if $v.Authenticator -}}if sauthc, ok := {{ $vPrefix }}AuthScheme.(setauthenticator); ok {
		app.Log().Debugf("Initializing authenticator for auth scheme '%s'", "{{ $k }}")
		if err := sauthc.SetAuthenticator(&{{ $v.Authenticator }}{}); err != nil {
			app.Log().Fatal(err)
		}
	}{{ end }}
	{{ if $v.Principal -}}if sprincipal, ok := {{ $vPrefix }}AuthScheme.(setprincipal); ok {
		app.Log().Debugf("Initializing principalprovider for auth scheme '%s'", "{{ $k }}")
		if err := sprincipal.SetPrincipalProvider(&{{ $v.Principal }}{}); err != nil {
			app.Log().Fatal(err)
		}
	}{{ end }}
	{{ if $v.Authorizer }}if sauthz, ok := {{ $vPrefix }}AuthScheme.(setauthorizer); ok {
		app.Log().Debugf("Initializing authorizer for auth scheme '%s'", "{{ $k }}")
		if err := sauthz.SetAuthorizer(&{{ $v.Authorizer }}{}); err != nil {
			app.Log().Fatal(err)
		}
	}{{ end }}
	{{ end -}}
	{{ end }}
}
`

const aahMainTemplate = `// Code generated by aah CLI, DO NOT EDIT
//
// aah framework v{{ .AahVersion }} - https://aahframework.org
// FILE: aah.go
// DESC: aah application entry point

package main

import (
	"bytes"
	"os"

	"aahframe.work"
	"aahframe.work/aruntime"
	_ "{{ .AppImportPath }}/app/generated"
)

func main() {
	app := aah.App()
	defer func() {
		if r := recover(); r != nil {
			st := aruntime.NewStacktrace(r, app.Config())
			buf := new(bytes.Buffer)
			st.Print(buf)
			app.Log().Error(buf.String())
		}
	}()

	if err := app.Run(os.Args); err != nil {
		app.Log().Error(err)
	}
}
`
