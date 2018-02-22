// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

const (
	typeWeb        = "web"
	typeAPI        = "api"
	storeCookie    = "cookie"
	storeFile      = "file"
	aahTmplExt     = ".atmpl"
	authForm       = "form"
	authBasic      = "basic"
	authGeneric    = "generic"
	authNone       = "none"
	basicFileRealm = "file-realm"
)

var (
	newCmd = cli.Command{
		Name:    "new",
		Aliases: []string{"n"},
		Usage:   "Create new aah 'web' or 'api' application (interactive)",
		Description: `aah new command is an interactive program to assist you to quick start aah application.

	Just provide your inputs based on your use case to generate base structure to kick
	start your development.

	Go to https://docs.aahframework.org to learn more and customize your aah application.
	`,
		Action: newAction,
	}

	reader = bufio.NewReader(os.Stdin)
)

func newAction(c *cli.Context) error {
	fmt.Println("\nWelcome to interactive way to create your aah application, press ^C to exit :)")
	fmt.Println()
	fmt.Println("Based on your inputs, aah CLI tool generates the aah application structure for you.")

	// Collect data
	importPath := getImportPath(reader)
	appType := getAppType(reader)
	authScheme := getAuthScheme(reader, appType)
	basicAuthMode := getBasicAuthMode(reader, authScheme)
	passwordEncoder := getPasswordHashAlgorithm(reader, authScheme)
	sessionStore := getSessionInfo(reader, appType, authScheme)

	// Process it
	appDir := filepath.Join(gosrcDir, filepath.FromSlash(importPath))
	appName := filepath.Base(appDir)
	appSessionFilepath := filepath.ToSlash(filepath.Join(appDir, "sessions"))
	data := map[string]interface{}{
		"AppName":                 appName,
		"AppType":                 appType,
		"AppImportPath":           importPath,
		"AppAuthScheme":           authScheme,
		"AppBasicAuthMode":        basicAuthMode,
		"AppPasswordEncoder":      passwordEncoder,
		"AppSessionStore":         sessionStore,
		"AppSessionFileStorePath": appSessionFilepath,
		"AppSessionSignKey":       ess.SecureRandomString(64),
		"AppSessionEncKey":        ess.SecureRandomString(32),
		"AppAntiCSRFSignKey":      ess.SecureRandomString(64),
		"AppAntiCSRFEncKey":       ess.SecureRandomString(32),
		"TmplDemils":              "{{.}}",
	}

	if basicAuthMode == basicFileRealm {
		data["AppBasicAuthFileRealmPath"] = filepath.Join(appDir, "config", "basic-realm.conf")
	} else {
		data["AppBasicAuthFileRealmPath"] = "/path/to/basic-realm.conf"
	}

	if err := createAahApp(appDir, appType, data); err != nil {
		fatal(err)
	}

	fmt.Printf("\nYour aah %s application was created successfully at '%s'\n", appType, appDir)
	fmt.Printf("You shall run your application via the command: 'aah run --importpath %s'\n", importPath)
	fmt.Println("\nGo to https://docs.aahframework.org to learn more and customize your aah application.")

	if basicAuthMode == basicFileRealm {
		fmt.Println("\nNext step:")
		fmt.Println("\tCreate basic auth realm file per your application requirements.")
		fmt.Println("\tRefer to 'https://docs.aahframework.org/authentication.html#basic-auth-file-realm-format' to create basic auth realm file.")
	}
	fmt.Println()
	return nil
}

func readInput(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Error(err)
		return ""
	}
	return strings.TrimSpace(input)
}

func getImportPath(reader *bufio.Reader) string {
	var importPath string
	for {
		importPath = filepath.ToSlash(readInput(reader, "\nEnter your application import path: "))
		if !ess.IsStrEmpty(importPath) {
			if ess.IsImportPathExists(importPath) {
				log.Errorf("Given import path '%s' already exists", importPath)
				importPath = ""
				continue
			}
			break
		}
	}
	return strings.Replace(importPath, " ", "-", -1)
}

func getAppType(reader *bufio.Reader) string {
	var appType string
	for {
		appType = readInput(reader, "\nChoose your application type (web or api), default is 'web': ")
		if ess.IsStrEmpty(appType) || appType == typeWeb || appType == typeAPI {
			break
		} else {
			log.Error("Unsupported new aah application type, choose either 'web or 'api'")
			appType = ""
		}
	}
	if ess.IsStrEmpty(appType) {
		appType = typeWeb
	}
	return appType
}

func getAuthScheme(reader *bufio.Reader, appType string) string {
	var (
		authScheme  string
		schemeNames string
	)

	if appType == typeWeb {
		schemeNames = "form, basic"
	} else if appType == typeAPI {
		schemeNames = "basic, generic"
	}

	for {
		authScheme = readInput(reader, fmt.Sprintf("\nChoose your application Auth Scheme (%v), default is 'none': ", schemeNames))
		if isAuthSchemeSupported(authScheme) {
			if ess.IsStrEmpty(authScheme) || authScheme == authNone ||
				(appType == typeWeb && (authScheme == authForm || authScheme == authBasic)) ||
				(appType == typeAPI && (authScheme == authGeneric || authScheme == authBasic)) {
				break
			} else {
				log.Errorf("Application type '%v' is not applicable with auth scheme '%v'", appType, authScheme)
				authScheme = ""
			}
		} else {
			log.Errorf("Unsupported Auth Scheme, choose either %v or 'none'", schemeNames)
			authScheme = ""
		}
	}

	if authScheme == authNone {
		authScheme = ""
	}

	return authScheme
}

func getBasicAuthMode(reader *bufio.Reader, authScheme string) string {
	var basicAuthMode string
	if authScheme == authBasic {
		for {
			basicAuthMode = readInput(reader, "\nChoose your basic auth mode (file-realm, dynamic), default is 'file-realm': ")
			if ess.IsStrEmpty(basicAuthMode) || basicAuthMode == "dynamic" {
				break
			} else {
				log.Error("Unsupported Basic auth mode")
				basicAuthMode = ""
			}
		}

		if ess.IsStrEmpty(basicAuthMode) {
			basicAuthMode = basicFileRealm
		}
	}

	return basicAuthMode
}

func getPasswordHashAlgorithm(reader *bufio.Reader, authScheme string) string {
	var authPasswordAlgorithm string
	if authScheme == authForm || authScheme == authBasic {
		for {
			authPasswordAlgorithm = readInput(reader, "\nChoose your password hash algorithm (bcrypt, scrypt, pbkdf2), default is 'bcrypt': ")

			if ess.IsStrEmpty(authPasswordAlgorithm) || authPasswordAlgorithm == "bcrypt" ||
				authPasswordAlgorithm == "scrypt" || authPasswordAlgorithm == "pbkdf2" {
				break
			} else {
				log.Error("Unsupported Password hash algorithm")
				authPasswordAlgorithm = ""
			}
		}

		if ess.IsStrEmpty(authPasswordAlgorithm) {
			authPasswordAlgorithm = "bcrypt"
		}
	}
	return authPasswordAlgorithm
}

func getSessionInfo(reader *bufio.Reader, appType, authScheme string) string {
	sessionStore := storeCookie

	if appType == typeWeb && (authScheme == authForm || authScheme == authBasic) {
		// Session Store
		for {
			sessionStore = readInput(reader, "\nChoose your session store (cookie or file), default is 'cookie': ")
			if ess.IsStrEmpty(sessionStore) || sessionStore == storeCookie || sessionStore == storeFile {
				break
			} else {
				log.Error("Unsupported session store type, choose either 'cookie or 'file")
				sessionStore = ""
			}
		}

		if ess.IsStrEmpty(sessionStore) {
			sessionStore = storeCookie
		}
	}

	return sessionStore
}

func createAahApp(appDir, appType string, data map[string]interface{}) error {
	aahToolsPath := getAahToolsPath()
	appTemplatePath := filepath.Join(aahToolsPath.Dir, "app-template")

	// app directory creation
	if err := ess.MkDirAll(appDir, permRWXRXRX); err != nil {
		fatal(err)
	}

	// aah.project
	processFile(appDir, appTemplatePath, filepath.Join(appTemplatePath, "aah.project.atmpl"), data)

	// gitignore
	processFile(appDir, appTemplatePath, filepath.Join(appTemplatePath, ".gitignore"), data)

	// source
	processSection(appDir, appTemplatePath, "app", data)

	// config
	processSection(appDir, appTemplatePath, "config", data)

	if typeWeb == appType {
		// i18n
		processSection(appDir, appTemplatePath, "i18n", data)

		// static
		processSection(appDir, appTemplatePath, "static", data)

		// views
		processSection(appDir, appTemplatePath, "views", data)
	}

	return nil
}

func processSection(destDir, srcDir, dir string, data map[string]interface{}) {
	files, _ := ess.FilesPath(filepath.Join(srcDir, dir), true)
	for _, v := range files {
		if strings.Contains(v, "/app/security/") {
			authScheme := data["AppAuthScheme"].(string)
			if !ess.IsStrEmpty(authScheme) && authScheme != authNone {
				if authScheme == authBasic {
					basicAuthMode := data["AppBasicAuthMode"].(string)
					if basicAuthMode == "dynamic" {
						processFile(destDir, srcDir, v, data)
					}
				} else {
					processFile(destDir, srcDir, v, data)
				}
			}
		} else {
			processFile(destDir, srcDir, v, data)
		}
	}
}

func processFile(destDir, srcDir, f string, data map[string]interface{}) {
	dfPath := getDestPath(destDir, srcDir, f)
	dfDir := filepath.Dir(dfPath)
	if !ess.IsFileExists(dfDir) {
		_ = ess.MkDirAll(dfDir, permRWXRXRX)
	}

	sf, _ := os.Open(f)
	df, _ := os.Create(dfPath)

	if strings.HasSuffix(f, aahTmplExt) {
		sfbytes, _ := ioutil.ReadAll(sf)
		if err := renderTmpl(df, string(sfbytes), data); err != nil {
			fatalf("Unable to process file '%s': %s", dfPath, err)
		}
	} else {
		_, _ = io.Copy(df, sf)
	}

	_ = ess.ApplyFileMode(dfPath, permRWRWRW)
	ess.CloseQuietly(sf, df)
}

func getDestPath(destDir, srcDir, v string) string {
	dpath := v[len(srcDir):]
	dpath = filepath.Join(destDir, dpath)
	if strings.HasSuffix(v, aahTmplExt) {
		dpath = dpath[:len(dpath)-len(aahTmplExt)]
	}
	return dpath
}

func isAuthSchemeSupported(authScheme string) bool {
	return ess.IsStrEmpty(authScheme) || authScheme == authForm || authScheme == authBasic ||
		authScheme == authGeneric || authScheme == authNone
}

func checkAndGenerateInitgoFile(importPath, baseDir string) {
	initGoFile := filepath.Join(baseDir, "app", "init.go")
	if !ess.IsFileExists(initGoFile) {
		log.Warn("In v0.10 'init.go' file introduced for evolving aah framework." +
			" Since its not found, generating 'init.go' file. Please add it to your version control.")

		aahToolsPath := getAahToolsPath()
		appTemplatePath := filepath.Join(aahToolsPath.Dir, "app-template")
		appType := typeAPI
		if ess.IsFileExists(filepath.Join(baseDir, "views")) {
			appType = typeWeb
		}
		data := map[string]interface{}{
			"AppType": appType,
		}

		processFile(baseDir, appTemplatePath, filepath.Join(appTemplatePath, "app", "init.go"+aahTmplExt), data)
	}
}

func getAahToolsPath() *build.Package {
	aahToolsPath, err := build.Import(path.Join(libImportPath("tools"), "aah"), "", build.FindOnly)
	if err != nil {
		fatal(err)
	}
	return aahToolsPath
}
