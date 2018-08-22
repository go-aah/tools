// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// aahframework.org/tools/aah source code and usage is governed by a MIT style
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

	"aahframe.work/aah"
	"aahframe.work/aah/ainsp"
	"aahframe.work/aah/config"
	"aahframe.work/aah/essentials"
	"aahframe.work/aah/router"
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

	appImportPaths := map[string]string{
		"aahframe.work/aah":            "aah",
		"aahframe.work/aah/aruntime":   "aruntime",
		"aahframe.work/aah/config":     "config",
		"aahframe.work/aah/essentials": "ess",
		"aahframe.work/aah/log":        "log",
	}

	// get all the types info referred aah framework context embedded
	appControllers := acntlr.FindTypeByEmbeddedType(aahImportPath + ".Context")
	appImportPaths = acntlr.CreateImportPaths(appControllers, appImportPaths)
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

var notExistRegex = regexp.MustCompile(`cannot find package "(.*)" in any of`)

// checkAndGetAppDeps method project dependencies is present otherwise
// it tries to get it if any issues it will return error. It internally uses
// go list command.
// 		go list -f '{{ join .Imports "\n" }}' aah-app/import/path/app/...
//
func checkAndGetAppDeps(appImportPath string, cfg *config.Config) error {
	debList := libDependencyImports(path.Join(appImportPath, "app", "..."))
	if len(debList) == 0 {
		return nil
	}

	args := append([]string{"list"}, debList...)
	b, _ := exec.Command(gocmd, args...).CombinedOutput()
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

const aahMainTemplate = `// Code generated by aah CLI, DO NOT EDIT
//
// aah framework v{{.AahVersion}} - https://aahframework.org
// FILE: aah.go
// DESC: aah application entry point

package main

import (
	"bytes"
	"flag"
	"path/filepath"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"syscall"
	{{ if .AppSecurity }}
	"aahframe.work/aah/security/authc"
	"aahframe.work/aah/security/authz"{{ end }}{{ range $k, $v := $.AppImportPaths }}
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
	cpath, err := filepath.Abs(*configPath)
	if err != nil {
		log.Errorf("Unable to resolve external config: %s", *configPath)
	}

	externalConfig, err := config.LoadFile(cpath)
	if err != nil {
		log.Errorf("Unable to load external config: %s", cpath)
	}

	log.Infof("Merging external config[%s] into aah application[%s]", cpath, aah.AppName())
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
		log.Error(err)
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
		log.Error(err)
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

	// display application information
	if *version {
		fmt.Printf("%-16s: %s\n", "Binary Name", aah.AppBuildInfo().BinaryName)
		fmt.Printf("%-16s: %s\n", "Version", aah.AppBuildInfo().Version)
		fmt.Printf("%-16s: %s\n", "Build Timestamp", aah.AppBuildInfo().Date)
		fmt.Printf("%-16s: %s\n", "aah Version", aah.Version)
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
	secMgr := aah.AppSecurityManager()
	{{- range $k, $v := $.AppSecurity }}{{ $vPrefix := (variablename $k)  }}
	{{ $vPrefix }}AuthScheme := secMgr.AuthScheme("{{ $k }}")
	{{ if $v.Authenticator -}}if sauthc, ok := {{ $vPrefix }}AuthScheme.(setauthenticator); ok {
		aah.AppLog().Debugf("Initializing authenticator for auth scheme '%s'", "{{ $k }}")
		if err := sauthc.SetAuthenticator(&{{ $v.Authenticator }}{}); err != nil {
			aah.AppLog().Fatal(err)
		}
	}{{ end }}
	{{ if $v.Principal -}}if sprincipal, ok := {{ $vPrefix }}AuthScheme.(setprincipal); ok {
		aah.AppLog().Debugf("Initializing principalprovider for auth scheme '%s'", "{{ $k }}")
		if err := sprincipal.SetPrincipalProvider(&{{ $v.Principal }}{}); err != nil {
			aah.AppLog().Fatal(err)
		}
	}{{ end }}
	{{ if $v.Authorizer }}if sauthz, ok := {{ $vPrefix }}AuthScheme.(setauthorizer); ok {
		aah.AppLog().Debugf("Initializing authorizer for auth scheme '%s'", "{{ $k }}")
		if err := sauthz.SetAuthorizer(&{{ $v.Authorizer }}{}); err != nil {
			aah.AppLog().Fatal(err)
		}
	}{{ end }}
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
}
`
