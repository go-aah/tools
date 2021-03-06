// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aahframe.work/console"
	"aahframe.work/essentials"
)

var (
	newCmd = console.Command{
		Name:    "new",
		Aliases: []string{"n"},
		Usage:   "Creates new aah 'web', 'api' or 'websocket' application (interactive)",
		Description: `Command 'new' is an interactive program to assist you to quick start aah application.

	Just provide your inputs based on your use case to generate base structure to kickstart 
	your development.

	Application templates are kept at '$HOME/.aah/app-templates'.

	Go to https://docs.aahframework.org to learn more and customize your aah application.`,
		Action: newAction,
	}

	reader = bufio.NewReader(os.Stdin)
)

func newAction(c *console.Context) error {
	cliLog = initCLILogger(nil)
	fmt.Println("\nWelcome to interactive way to create your aah application, press ^C to exit :)")
	fmt.Println()
	fmt.Println("Based on your inputs, aah CLI generates the aah application structure for you.")

	// Collect inputs for aah app creation
	importPath := collectImportPath(reader)
	appDir := collectAppDir(reader, importPath)
	appType := collectAppType(reader)

	// Depends on application type choice, collect subsequent inputs
	app := &appTmplData{
		ImportPath:     importPath,
		BaseDir:        appDir,
		Type:           appType,
		TmplDelimLeft:  "{{",
		TmplDelimRight: "}}",
	}

	switch appType {
	case typeWeb:
		collectInputsForWebApp(c, app)
	case typeAPI:
		collectInputsForAPIApp(c, app)
	}

	// Process it
	app.Name = filepath.Base(app.BaseDir)
	app.SessionFileStorePath = filepath.ToSlash(filepath.Join(app.BaseDir, "sessions"))

	if app.IsBasicAuthFileRealm() {
		app.BasicAuthFileRealmPath = filepath.ToSlash(filepath.Join(app.BaseDir, "config", "basic-auth-realm.conf"))
	} else {
		app.BasicAuthFileRealmPath = "/path/to/basic-realm.conf"
	}

	if err := createAahApp(app.BaseDir, map[string]interface{}{
		"App": app,
	}); err != nil {
		logFatal(err)
	}

	fmt.Printf("\nYour aah %s application was created successfully at '%s'\n", app.Type, app.BaseDir)
	fmt.Println("You shall run your application via the command 'aah run' from application base directory.")
	fmt.Println("\nGo to https://docs.aahframework.org to learn more and customize your aah application.")

	_ = aahInventory.AddProject(app.ImportPath, app.BaseDir)

	if app.BasicAuthMode == basicFileRealm {
		fmt.Println("\nNext step:")
		fmt.Println("\tSample Basic Auth file-realm have been created at", app.BasicAuthFileRealmPath)
		fmt.Println("\tRefer to 'https://docs.aahframework.org/auth-schemes/basic.html' and update realm file per your application requirements.")
	}
	fmt.Println()
	return nil
}

func readInput(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		logError(err)
		return ""
	}
	return strings.TrimSpace(input)
}

func collectImportPath(reader *bufio.Reader) string {
	var importPath string
	for {
		importPath = filepath.ToSlash(readInput(reader, "\nEnter your application import path: "))
		if !ess.IsStrEmpty(importPath) {
			if m := aahInventory.Lookup(importPath); m != nil {
				logErrorf("Given import path '%s' already exists at '%s'", importPath, m.Dir)
				importPath = ""
				continue
			} else {
				if ess.IsImportPathExists(importPath) {
					logErrorf("Given import path '%s' already exists in GOPATH", importPath)
					importPath = ""
					continue
				}
			}
			break
		}
	}
	return strings.Replace(importPath, " ", "-", -1)
}

func collectAppDir(reader *bufio.Reader, importPath string) string {
	var dir string
	for {
		dir = filepath.ToSlash(readInput(reader, "\nEnter your application location: "))
		dir = filepath.Clean(dir)
		if !ess.IsStrEmpty(dir) {
			dir = filepath.Join(dir, path.Base(importPath))
			if inferInsideGopath(dir) {
				logError("Given directory is inside the GOPATH, it is highly recommneded to keep aah project outside the GOPATH")
				dir = ""
				continue
			}
			if ess.IsFileExists(dir) {
				logErrorf("Given directory already exists at '%s'", dir)
				dir = ""
				continue
			}
			break
		}
	}
	return dir
}

func collectAppType(reader *bufio.Reader) string {
	var appType string
	for {
		appType = strings.ToLower(readInput(reader, "\nChoose your application type (web, api or websocket), default is 'web': "))
		if ess.IsStrEmpty(appType) || appType == typeWeb || appType == typeAPI || appType == typeWebSocket {
			break
		} else {
			logError("Unsupported new aah application type, choose either 'web', 'api' or 'websocket'")
			appType = ""
		}
	}
	if ess.IsStrEmpty(appType) {
		appType = typeWeb
	}
	return appType
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Collecting inputs for Web App
//______________________________________________________________________________

func collectInputsForWebApp(c *console.Context, app *appTmplData) {
	viewEngine(reader, app)

	authScheme(reader, app)

	if app.AuthScheme == authBasic {
		basicAuthMode(reader, app)
	}

	passwordHashAlgorithm(reader, app)

	sessionInfo(reader, app)

	// In the web application user may like to have API also WebSocket within it.
	collectAppSubTypesChoice(c, reader, app)

	app.CORSEnable = collectYesOrNo(reader, "Would you like to enable CORS? [y/N]")
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Collecting inputs for API App
//______________________________________________________________________________

func collectInputsForAPIApp(c *console.Context, app *appTmplData) {
	authScheme(reader, app)

	if app.AuthScheme == authBasic {
		basicAuthMode(reader, app)
	}

	passwordHashAlgorithm(reader, app)

	app.CORSEnable = collectYesOrNo(reader, "Would you like to enable CORS? [y/N]")
}

func collectAppSubTypesChoice(c *console.Context, reader *bufio.Reader, app *appTmplData) {
	app.SubTypes = make([]string, 0)

	// API choice
	choice := collectYesOrNo(reader, "Would you like to add API (/api/v1/*) within your Web App? [y/N]")
	if choice {
		app.SubTypes = append(app.SubTypes, typeAPI)
	}

	// WebSocket choice
	choice = collectYesOrNo(reader, "Would you like to add WebSocket (/ws/*) within your Web App? [y/N]")
	if choice {
		app.SubTypes = append(app.SubTypes, typeWebSocket)
	}
}

func viewEngine(reader *bufio.Reader, app *appTmplData) {
	builtInViewEngines := []string{"go"}
	var engine string
	for {
		engine = strings.ToLower(readInput(reader, fmt.Sprintf("\nChoose your application View Engine (%s), default is 'go': ",
			strings.Join(builtInViewEngines, ", "))))
		if ess.IsStrEmpty(engine) || ess.IsSliceContainsString(builtInViewEngines, engine) {
			break
		} else {
			logErrorf("Unsupported View Engine")
			engine = ""
		}
	}

	switch engine {
	case "pug":
		app.ViewEngine = "pug"
		app.ViewFileExt = ".pug"
	default:
		app.ViewEngine = "go"
		app.ViewFileExt = ".html"
	}
}

func authScheme(reader *bufio.Reader, app *appTmplData) {
	var schemeNames string

	if app.IsWebApp() {
		schemeNames = "form, basic"
	} else if app.IsAPIApp() {
		schemeNames = "basic, generic"
	}

	for {
		app.AuthScheme = strings.ToLower(readInput(reader, fmt.Sprintf("\nChoose your application Auth Scheme (%v), default is 'none': ", schemeNames)))
		if isAuthSchemeSupported(app.AuthScheme) {
			if ess.IsStrEmpty(app.AuthScheme) || app.AuthScheme == authNone ||
				app.IsAuthSchemeForWeb() || app.IsAuthSchemeForAPI() {
				break
			} else {
				logErrorf("Application type '%v' is not applicable with auth scheme '%v'", app.Type, app.AuthScheme)
				app.AuthScheme = ""
			}
		} else {
			logErrorf("Unsupported Auth Scheme")
			app.AuthScheme = ""
		}
	}

	if app.AuthScheme == authNone {
		app.AuthScheme = ""
	}
}

func basicAuthMode(reader *bufio.Reader, app *appTmplData) {
	for {
		app.BasicAuthMode = strings.ToLower(readInput(reader, "\nChoose your basic auth mode (file-realm, dynamic), default is 'file-realm': "))
		if ess.IsStrEmpty(app.BasicAuthMode) || app.BasicAuthMode == "dynamic" {
			break
		} else {
			logError("Unsupported Basic auth mode")
			app.BasicAuthMode = ""
		}
	}

	if ess.IsStrEmpty(app.BasicAuthMode) {
		app.BasicAuthMode = basicFileRealm
	}
}

func passwordHashAlgorithm(reader *bufio.Reader, app *appTmplData) {
	if app.AuthScheme == authForm || app.AuthScheme == authBasic {
		for {
			app.PasswordEncoderAlgo = strings.ToLower(readInput(reader, "\nChoose your password hash algorithm (bcrypt, scrypt, pbkdf2), default is 'bcrypt': "))
			if ess.IsStrEmpty(app.PasswordEncoderAlgo) || app.PasswordEncoderAlgo == "bcrypt" ||
				app.PasswordEncoderAlgo == "scrypt" || app.PasswordEncoderAlgo == "pbkdf2" {
				break
			} else {
				logError("Unsupported Password hash algorithm")
				app.PasswordEncoderAlgo = ""
			}
		}

		if ess.IsStrEmpty(app.PasswordEncoderAlgo) {
			app.PasswordEncoderAlgo = "bcrypt"
		}
	}
}

func sessionInfo(reader *bufio.Reader, app *appTmplData) {
	if app.IsAuthSchemeForWeb() {
		for {
			app.SessionStore = strings.ToLower(readInput(reader, "\nChoose your session store (cookie or file), default is 'cookie': "))
			if ess.IsStrEmpty(app.SessionStore) || app.SessionStore == storeCookie || app.SessionStore == storeFile {
				break
			} else {
				logError("Unsupported session store type")
				app.SessionStore = ""
			}
		}

		if ess.IsStrEmpty(app.SessionStore) {
			app.SessionStore = storeCookie
		}
	}
}

func collectYesOrNo(reader *bufio.Reader, msg string) bool {
	var input string
	for {
		input = strings.ToLower(readInput(reader, "\n"+msg+": "))
		if ess.IsStrEmpty(input) {
			input = "n"
		}

		if input == "y" || input == "n" {
			break
		} else {
			logError("Invalid choice, please provide [Y]es or [N]o")
			input = ""
		}
	}
	return input == "y"
}

type file struct {
	src, dst string
}

func createAahApp(appDir string, data map[string]interface{}) error {
	app := data["App"].(*appTmplData)
	appBaseDir := app.BaseDir
	appTmplBaseDir := inferAppTmplBaseDir()
	if ess.IsStrEmpty(appTmplBaseDir) {
		logFatal("Unable to find aah app template at $HOME/.aah/app-templates")
	}

	// app directory creation
	if err := ess.MkDirAll(appDir, permRWXRXRX); err != nil {
		logFatal(err)
	}

	files := make([]file, 0)

	// aah.project
	files = append(files, file{
		src: filepath.Join(appTmplBaseDir, "aah.project.atmpl"),
		dst: filepath.Join(appBaseDir, "aah.project.atmpl"),
	})

	// go.mod
	files = append(files, file{
		src: filepath.Join(appTmplBaseDir, "go.mod.atmpl"),
		dst: filepath.Join(appBaseDir, "go.mod.atmpl"),
	})

	// .gitignore
	files = append(files, file{
		src: filepath.Join(appTmplBaseDir, ".gitignore"),
		dst: filepath.Join(appBaseDir, ".gitignore"),
	})

	if app.IsBasicAuthFileRealm() {
		files = append(files, file{
			src: filepath.Join(appTmplBaseDir, "misc", "basic-auth-realm.conf"),
			dst: app.BasicAuthFileRealmPath,
		})
	}

	// source
	files = append(files, sourceTmplFiles(app, appTmplBaseDir, appBaseDir)...)

	// config
	files = append(files, configTmplFiles(app.Type, appTmplBaseDir, appBaseDir)...)

	if app.IsWebApp() {
		// i18n
		files = append(files, tmplFiles(filepath.Join(appTmplBaseDir, "i18n"), appTmplBaseDir, appBaseDir, true)...)

		// static
		files = append(files, tmplFiles(filepath.Join(appTmplBaseDir, "static"), appTmplBaseDir, appBaseDir, true)...)

		// views
		files = append(files, viewTmplFiles(app.ViewEngine, appTmplBaseDir, appBaseDir)...)
	}

	// processing app template files
	for _, f := range files {
		processFile(appBaseDir, f, data)
	}

	return nil
}

func configTmplFiles(appType, appTmplBaseDir, appBaseDir string) []file {
	srcDir := filepath.Join(appTmplBaseDir, "config")
	flist, _ := ess.FilesPath(srcDir, true)
	files := []file{}
	for _, f := range flist {
		if appType == typeWebSocket && strings.HasSuffix(f, "security.conf.atmpl") {
			continue
		}
		files = append(files, file{src: f, dst: filepath.Join(appBaseDir, f[len(appTmplBaseDir):])})
	}
	return files
}

func sourceTmplFiles(app *appTmplData, appTmplBaseDir, appBaseDir string) []file {
	files := []file{}

	fn := func(srcDir string, recur bool) {
		flist, _ := ess.FilesPath(srcDir, recur)
		for _, f := range flist {
			files = append(files, file{src: f, dst: filepath.Join(appBaseDir, f[len(appTmplBaseDir):])})
		}
	}

	// /app
	fn(filepath.Join(appTmplBaseDir, "app"), false)

	// /app/controllers
	if app.IsWebApp() || app.IsAPIApp() {
		fn(filepath.Join(appTmplBaseDir, "app", "controllers"), false)

	}

	if app.IsAPIApp() {
		fn(filepath.Join(appTmplBaseDir, "app", "controllers", "v1"), false)
	}

	if app.IsSubTypeAPI() {
		files = append(files, file{
			src: filepath.Join(appTmplBaseDir, filepath.FromSlash("app/controllers/v1/value.go.atmpl")),
			dst: filepath.Join(appBaseDir, filepath.FromSlash("app/controllers/api/v1/value.go")),
		})
	}

	// /app/websockets
	if app.IsWebSocketApp() || app.IsSubTypeWebSocket() {
		fn(filepath.Join(appTmplBaseDir, "app", "websockets"), true)
	}

	// /app/models
	files = append(files, file{
		src: filepath.Join(appTmplBaseDir, filepath.FromSlash("app/models/greet.go")),
		dst: filepath.Join(appBaseDir, filepath.FromSlash("app/models/greet.go")),
	})
	if app.IsAPIApp() || app.IsSubTypeAPI() {
		files = append(files, file{
			src: filepath.Join(appTmplBaseDir, filepath.FromSlash("app/models/value.go")),
			dst: filepath.Join(appBaseDir, filepath.FromSlash("app/models/value.go")),
		})
	}

	// /app/security
	if app.IsSecurityEnabled() && app.BasicAuthMode != basicFileRealm {
		fn(filepath.Join(appTmplBaseDir, "app", "security"), true)
	}

	return files
}

func viewTmplFiles(engName, appTmplBaseDir, appBaseDir string) []file {
	srcDir := filepath.Join(appTmplBaseDir, "views", engName)
	flist, _ := ess.FilesPath(srcDir, true)
	files := []file{}
	for _, f := range flist {
		files = append(files, file{src: f, dst: filepath.Join(appBaseDir, "views", f[len(srcDir):])})
	}
	return files
}

func tmplFiles(srcDir, appTmplBaseDir, appBaseDir string, recur bool) []file {
	flist, _ := ess.FilesPath(srcDir, recur)
	files := []file{}
	for _, f := range flist {
		files = append(files, file{src: f, dst: filepath.Join(appBaseDir, f[len(appTmplBaseDir):])})
	}
	return files
}

func processFile(appBaseDir string, f file, data map[string]interface{}) {
	dst := strings.TrimSuffix(f.dst, aahTmplExt)

	// create dst dir if not exists
	dstDir := filepath.Dir(dst)
	if !ess.IsFileExists(dstDir) {
		_ = ess.MkDirAll(dstDir, permRWXRXRX)
	}

	// open src and create dst
	sf, _ := os.Open(f.src)
	df, _ := os.Create(dst)
	defer ess.CloseQuietly(df, sf)

	// render or write it directly
	if strings.HasSuffix(f.src, aahTmplExt) {
		sfbytes, _ := ioutil.ReadAll(sf)
		var buf bytes.Buffer
		if err := renderTmpl(&buf, string(sfbytes), data); err != nil {
			logFatalf("Unable to process file '%s': %s", dst, err)
		}
		var err error
		b := buf.Bytes()
		if strings.HasSuffix(dst, ".go") {
			if b, err = format.Source(b); err != nil {
				logFatalf("aah '%s' file format source error: %s", dst, err)
			}
		}
		_, _ = io.Copy(df, bytes.NewReader(b))
	} else {
		_, _ = io.Copy(df, sf)
	}

	_ = ess.ApplyFileMode(dst, permRWRWRW)
}

func isAuthSchemeSupported(authScheme string) bool {
	return ess.IsStrEmpty(authScheme) || authScheme == authForm || authScheme == authBasic ||
		authScheme == authGeneric || authScheme == authNone
}

const templateBranchName = "0.12.x"

func inferAppTmplBaseDir() string {
	aahBasePath := aahPath()
	baseDir := filepath.Join(aahBasePath, "app-templates", "generic")
	gitBaseDir := filepath.Dir(baseDir)
	if ess.IsFileExists(gitBaseDir) {
		err1 := gitPull(gitBaseDir)
		err2 := gitCheckout(gitBaseDir, templateBranchName)
		if err1 == nil && err2 == nil {
			return baseDir
		}
	}

	if err := os.RemoveAll(gitBaseDir); err != nil {
		logError(err)
		return ""
	}
	tmplRepo := "https://github.com/go-aah/app-templates.git"
	cliLog.Infof("Downloading aah quick start app templates from %s", tmplRepo)
	gitArgs := []string{"clone", tmplRepo, gitBaseDir}
	if _, err := execCmd(gitcmd, gitArgs, false); err != nil {
		logErrorf("Unable to download aah app-template from %s", tmplRepo)
		return ""
	}
	if err := gitCheckout(gitBaseDir, templateBranchName); err != nil {
		logError(err)
		return ""
	}
	return baseDir
}
