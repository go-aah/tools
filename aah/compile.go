// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"aahframework.org/aah.v0"
	"aahframework.org/ainsp.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/router.v0"
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
	appBaseDir := aah.AppBaseDir()
	appImportPath := aah.AppImportPath()
	appCodeDir := filepath.Join(appBaseDir, "app")
	appControllersPath := filepath.Join(appCodeDir, "controllers")
	appWebSocketsPath := filepath.Join(appCodeDir, "websockets")
	appBuildDir := filepath.Join(appBaseDir, "build")

	appName := projectCfg.StringDefault("name", aah.AppName())
	cliLog.Infof("Compile starts for '%s' [%s]", appName, appImportPath)

	// excludes for Go AST processing
	excludes, _ := projectCfg.StringList("build.ast_excludes")

	// get all configured Controllers with action info
	registeredActions := aah.AppRouter().RegisteredActions()

	// Go AST processing for Controllers
	acntlr, errs := ainsp.Inspect(appControllersPath, ess.Excludes(excludes), registeredActions)
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

	// get all the types info referred aah framework context embedded
	appControllers := acntlr.FindTypeByEmbeddedType(fmt.Sprintf("%s.Context", libImportPath("aah")))
	appImportPaths := acntlr.CreateImportPaths(appControllers, map[string]string{})
	appSecurity := appSecurity(aah.AppConfig(), appImportPaths)

	// Go AST processing for WebSockets
	registeredWSActions := aah.AppRouter().RegisteredWSActions()
	wsc, errs := ainsp.Inspect(appWebSocketsPath, ess.Excludes(excludes), registeredWSActions)
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

	appWebSockets := wsc.FindTypeByEmbeddedType(fmt.Sprintf("%s.Context", libImportPath("ws")))
	appImportPaths = wsc.CreateImportPaths(appWebSockets, appImportPaths)

	if len(appControllers) == 0 && len(appWebSockets) == 0 {
		return "", fmt.Errorf("It seems your application have zero controller or websocket")
	}

	if len(appControllers) > 0 || len(appWebSockets) > 0 {
		appImportPaths[libImportPath("ainsp")] = "ainsp"
	}

	// prepare aah application version and build date
	appVersion := getAppVersion(appBaseDir, projectCfg)
	appBuildDate := getBuildDate()

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

	// clean previously auto generated files
	cleanupAutoGenFiles(appBaseDir)

	if err := generateSource(appCodeDir, "aah.go", aahMainTemplate, map[string]interface{}{
		"AppTargetCmd":   args.Cmd,
		"AppProxyPort":   args.ProxyPort,
		"AahVersion":     aah.Version,
		"AppImportPath":  appImportPath,
		"AppVersion":     appVersion,
		"AppBuildDate":   appBuildDate,
		"AppBinaryName":  appBinaryName,
		"AppControllers": appControllers,
		"AppWebSockets":  appWebSockets,
		"AppImportPaths": appImportPaths,
		"AppSecurity":    appSecurity,
		"AppIsPackaged":  args.AppPack,
		"AppIsEmbedded":  args.AppEmbed,
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
		if err := ess.MkDirAll(dir, 0644); err != nil {
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
		for _, pkg := range strings.Fields(line) {
			if ess.IsStrEmpty(pkg) || ess.IsImportPathExists(pkg) {
				continue
			}
			notExistsPkgs = append(notExistsPkgs, pkg)
		}

		if cfg.BoolDefault("build.dep_get", true) && len(notExistsPkgs) > 0 {
			cliLog.Info("Getting application dependencies ...")
			if err := goGet(notExistsPkgs...); err != nil {
				return err
			}
		} else if len(notExistsPkgs) > 0 {
			return fmt.Errorf("Below application dependencies does not exist, "+
				"enable 'build.dep_get=true' in 'aah.project' for auto fetch\n---> %s",
				strings.Join(notExistsPkgs, "\n---> "))
		}
	}

	return nil
}

func appSecurity(appCfg *config.Config, appImportPaths map[string]string) map[string]interface{} {
	securityInfo := make(map[string]interface{})
	importPathPrefix := path.Join(aah.AppImportPath(), "app")
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
			Authorizer    string
		}{}

		// Authenticator
		authenticator := appCfg.StringDefault(keyPrefixAuthSchemeCfg+".authenticator", "")
		if !ess.IsStrEmpty(authenticator) {
			authSchemeInfo.Authenticator = prepareAuthAlias(
				keyAuthScheme+"sec", authenticator, importPathPrefix, appImportPaths)
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

const aahMainTemplate = `// Code generated by aah CLI, DO NOT EDIT
//
// aah framework v{{.AahVersion}} - https://aahframework.org
// FILE: aah.go
// DESC: aah application entry point

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"syscall"

	"aahframework.org/aah.v0"
	"aahframework.org/aruntime.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"{{ range $k, $v := $.AppImportPaths }}
	{{ $v }} "{{ $k }}"{{ end }}
)

var (
	// Define aah application binary flags
	configPath = flag.String("config", "", "Absolute path of external config file.")
	list       = flag.String("list", "", "Prints the embedded file/directory path that matches the given regex pattern.")
	profile    = flag.String("profile", "", "Environment profile name to activate. For e.g.: dev, qa, prod, etc.")
	version    = flag.Bool("version", false, "Prints the aah application binary name, version and build timestamp.")
	_          = reflect.Invalid
)

func MergeSuppliedConfig(_ *aah.Event) {
	externalConfig, err := config.VFSLoadFile(aah.AppVFS(), *configPath)
	if err != nil {
		log.Fatalf("Unable to load external config: %s", *configPath)
	}

	log.Debug("Merging external config into aah application config")
	if err := aah.AppConfig().Merge(externalConfig); err != nil {
		log.Errorf("Unable to merge external config into aah application[%s]: %s", aah.AppName(), err)
	}
}

func ActivateAppEnvProfile(_ *aah.Event) {
	aah.AppConfig().SetString("env.active", *profile)
}

func PrintFilepath(pattern string) {
	if !aah.AppVFS().IsEmbeddedMode() {
		fmt.Println("'"+aah.AppBuildInfo().BinaryName + "' binary does not have embedded files.")
		return
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Println("ERROR", err)
		return
	}

	if err := aah.AppVFS().Walk(aah.AppVirtualBaseDir(),
		func(fpath string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if regex.MatchString(fpath) {
				fmt.Println(fpath)
			}

			return nil
		}); err != nil {
		fmt.Println("ERROR", err)
	}
}

{{ if eq .AppTargetCmd "RunCmd" -}}
{{ if .AppProxyPort -}}
func RunCmdSetAppProxyPort(e *aah.Event) {
	aah.AppConfig().SetString("server.proxyport", "{{ .AppProxyPort }}")
}
{{- end }}
{{- end }}

func main() {
	defer func() {
		if r := recover(); r != nil {
			st := aruntime.NewStacktrace(r, aah.AppConfig())
			buf := new(bytes.Buffer)
			st.Print(buf)
			log.Error(buf.String())
		}
	}()

	flag.Parse()

	aah.SetAppBuildInfo(&aah.BuildInfo{
		BinaryName: "{{ .AppBinaryName }}",
		Version:    "{{ .AppVersion }}",
		Date:       "{{ .AppBuildDate }}",
	})

	{{ if .AppIsPackaged }}aah.SetAppPackaged({{ .AppIsPackaged }}){{ end }}
	{{ if .AppIsEmbedded }}
	// Set app vfs into embedded mode
	aah.AppVFS().SetEmbeddedMode(){{ end }}

	// display application information
	if *version {
		fmt.Printf("%-16s: %s\n", "Binary Name", aah.AppBuildInfo().BinaryName)
		fmt.Printf("%-16s: %s\n", "Version", aah.AppBuildInfo().Version)
		fmt.Printf("%-16s: %s\n", "Build Timestamp", aah.AppBuildInfo().Date)
		return
	}

	if !ess.IsStrEmpty(*list) {
		PrintFilepath(*list)
		return
	}

	// Apply supplied external config file
	if !ess.IsStrEmpty(*configPath) {
		aah.OnInit(MergeSuppliedConfig)
	}

	// Activate environment profile
	if !ess.IsStrEmpty(*profile) {
		aah.OnInit(ActivateAppEnvProfile)
	}

	log.Infof("aah framework v%s, requires ≥ go1.8", aah.Version)

	if err := aah.Init("{{ .AppImportPath }}"); err != nil {
		log.Fatal(err)
	}

	{{ if gt (len .AppControllers) 0 -}}
	// Adding all the application controllers which refers 'aah.Context' directly
	// or indirectly from app/controllers/** {{ range $i, $c := .AppControllers }}
	aah.AddController((*{{ index $.AppImportPaths .ImportPath }}.{{ .Name }})(nil), []*ainsp.Method{ {{ range .Methods }}
		{Name: "{{ .Name }}"{{ if gt (len .Parameters) 0 }}, Parameters: []*ainsp.Parameter{ {{ range .Parameters }}
	    {Name: "{{ .Name }}", Type: reflect.TypeOf((*{{ .Type.Name }})(nil))},{{- end }}
		}{{ end }}},{{ end }}
	}){{- end }}
	{{ end -}}

	{{ if gt (len .AppWebSockets) 0 -}}
	// Adding all the application websockets which refers 'ws.Context' directly
	// or indirectly from app/websockets/** {{ range $i, $c := .AppWebSockets }}
	aah.AddWebSocket((*{{ index $.AppImportPaths .ImportPath }}.{{ .Name }})(nil), []*ainsp.Method{ {{ range .Methods }}
		{Name: "{{ .Name }}"{{ if gt (len .Parameters) 0 }}, Parameters: []*ainsp.Parameter{ {{ range .Parameters }}
	    {Name: "{{ .Name }}", Type: reflect.TypeOf((*{{ .Type.Name }})(nil))},{{- end }}
	  }{{ end }}},{{ end }}
	}){{- end }}
	{{ end -}}

	{{ if .AppSecurity }}
	// Initialize application security auth schemes - Authenticator & Authorizer
	secMgr := aah.AppSecurityManager()
	{{- range $k, $v := $.AppSecurity }}
	{{ if $v.Authenticator -}}
	aah.AppLog().Debugf("Calling authenticator Init for auth scheme '%s'", "{{ $k }}")
	if err := secMgr.GetAuthScheme("{{ $k }}").SetAuthenticator(&{{ $v.Authenticator }}{}); err != nil {
		aah.AppLog().Fatal(err)
	}
	{{ end -}}
	{{ if $v.Authorizer -}}
	aah.AppLog().Debugf("Calling authorizer Init for auth scheme '%s'", "{{ $k }}")
	if err := secMgr.GetAuthScheme("{{ $k }}").SetAuthorizer(&{{ $v.Authorizer }}{}); err != nil {
		aah.AppLog().Fatal(err)
	}
	{{ end -}}
	{{ end -}}
	{{ end }}

	aah.AppLog().Info("aah application initialized successfully")

	{{ if eq .AppTargetCmd "RunCmd" -}}
	{{ if .AppProxyPort -}}
	aah.OnStart(RunCmdSetAppProxyPort)
	{{- end }}
	{{- end }}

	go aah.Start()

	// Listen to OS signal's SIGINT & SIGTERM for aah server Shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	sig := <-sc
	switch sig {
	case os.Interrupt:
		aah.AppLog().Warn("Interrupt signal (SIGINT) received")
	case syscall.SIGTERM:
		aah.AppLog().Warn("Termination signal (SIGTERM) received")
	}

	// Call aah shutdown
	aah.Shutdown()
	aah.AppLog().Info("aah application shutdown successful")

	// bye bye, see you later.
	os.Exit(0)
}
`
