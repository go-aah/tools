// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aahframe.work"
	"aahframe.work/console"
	"aahframe.work/essentials"
	"aahframe.work/log"
)

var generateCmd = console.Command{
	Name:    "generate",
	Aliases: []string{"g"},
	Usage:   "Generates boilerplate code, configurations, complement scripts (systemd, docker), etc.",
	Description: `Command generate increases productivity and helps developer on tedious tasks during application development.
  Such as boilerplate code, configuration files, complement scripts (systemd, docker), etc.

	To know more about available 'generate' sub commands:
		aah help generate

	To know more about individual sub-commands details:
		aah generate help script`,
	Subcommands: []console.Command{
		{
			Name:    "script",
			Aliases: []string{"s"},
			Usage:   "Generates complement scripts such as systemd, dockerize, etc.",
			Description: `Generates complement scripts such as systemd, dockerize, etc.

	Example of script command:
		aah generate script --name systemd --importpath github.com/user/appname`,
			Flags: []console.Flag{
				console.StringFlag{
					Name:  "name, n",
					Usage: "Provide script name such as 'systemd', 'docker', etc",
				},
			},
			Action: generateScriptsAction,
		},
	},
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Generate Subcommand - Script
//___________________________________

func generateScriptsAction(c *console.Context) error {
	if !isAahProject() {
		logFatalf("Please go to aah application base directory and run '%s'.", strings.Join(os.Args, " "))
	}

	scriptName := strings.TrimSpace(c.String("name"))
	if ess.IsStrEmpty(scriptName) {
		_ = console.ShowSubcommandHelp(c)
		return nil
	}

	var err error
	switch scriptName {
	case "systemd":
		err = generateSystemdScript(c)
	case "docker":
		err = generateDockerScript(c)
	default:
		log.Error("Unsupported 'script' name, try one of these 'systemd', 'docker'")
	}

	if err != nil {
		logFatal(err)
	}

	return nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Implementation methods
//___________________________________

func generateSystemdScript(c *console.Context) error {
	importPath := appImportPath(c)
	if ess.IsStrEmpty(importPath) {
		logFatalf("Unable to infer import path, ensure you're in the application base directory")
	}
	chdirIfRequired(importPath)
	app := aah.App()
	if err := app.InitForCLI(importPath); err != nil {
		logFatal(err)
	}

	projectCfg := aahProjectCfg(app.BaseDir())
	cliLog = initCLILogger(projectCfg)

	cliLog.Infof("Loaded aah project file: %s\n", filepath.Join(app.BaseDir(), aahProjectIdentifier))

	fileName := fmt.Sprintf("%s.service", app.Name())
	destFile := filepath.Join(app.BaseDir(), fileName)
	if checkAndConfirmOverwrite(c, destFile) {
		return nil
	}

	data := map[string]interface{}{
		"AppName":    app.Name(),
		"FileName":   fileName,
		"CreateDate": time.Now().Format(time.RFC1123Z),
		"Desc":       fmt.Sprintf("%s application", app.Name()),
	}

	buf := &bytes.Buffer{}
	if err := renderTmpl(buf, aahSystemdScriptTemplate, data); err != nil {
		return fmt.Errorf("Unable to create systemd service file: %s", err)
	}
	if err := ioutil.WriteFile(destFile, buf.Bytes(), permRWXRXRX); err != nil {
		return fmt.Errorf("Unable to create systemd service file: %s", err)
	}

	cliLog.Infof("Generated 'systemd' service file at '%s'\n", destFile)
	cliLog.Infof("What's next, refer to https://docs.aahframework.org/getting-started-with-systemd.html#steps-to-configure-and-enable\n")

	return nil
}

func generateDockerScript(c *console.Context) error {
	importPath := appImportPath(c)
	if ess.IsStrEmpty(importPath) {
		logFatalf("Unable to infer import path, ensure you're in the application base directory")
	}
	app := aah.App()
	if err := app.InitForCLI(importPath); err != nil {
		logFatal(err)
	}
	projectCfg := aahProjectCfg(app.BaseDir())
	cliLog = initCLILogger(projectCfg)

	cliLog.Infof("Loaded aah project file: %s\n", filepath.Join(app.BaseDir(), aahProjectIdentifier))

	devFileName := "Dockerfile.dev"
	devDestFile := filepath.Join(app.BaseDir(), devFileName)
	if checkAndConfirmOverwrite(c, devDestFile) {
		return nil
	}

	prodFileName := "Dockerfile.prod"
	prodDestFile := filepath.Join(app.BaseDir(), prodFileName)
	if checkAndConfirmOverwrite(c, prodDestFile) {
		return nil
	}

	codeVersion := aah.Version
	if strings.HasSuffix(codeVersion, "-edge") {
		codeVersion = "edge"
	}

	devData := map[string]interface{}{
		"AppName":       app.Name(),
		"AppImportPath": app.ImportPath(),
		"FileName":      devFileName,
		"CreateDate":    time.Now().Format(time.RFC1123Z),
		"CodeVersion":   codeVersion,
	}

	prodData := map[string]interface{}{
		"AppName":       app.Name(),
		"AppImportPath": app.ImportPath(),
		"FileName":      prodFileName,
		"CreateDate":    time.Now().Format(time.RFC1123Z),
		"CodeVersion":   codeVersion,
	}

	buf := &bytes.Buffer{}
	if err := renderTmpl(buf, aahDockerDevScriptTemplate, devData); err != nil {
		return fmt.Errorf("Unable to create %s: %s", devFileName, err)
	}
	if err := ioutil.WriteFile(devDestFile, buf.Bytes(), permRWRWRW); err != nil {
		return fmt.Errorf("Unable to create %s: %s", devFileName, err)
	}
	_ = ess.ApplyFileMode(devDestFile, permRWRWRW)

	buf.Reset()
	if err := renderTmpl(buf, aahDockerProdScriptTemplate, prodData); err != nil {
		return fmt.Errorf("Unable to create %s: %s", prodFileName, err)
	}
	if err := ioutil.WriteFile(prodDestFile, buf.Bytes(), permRWRWRW); err != nil {
		return fmt.Errorf("Unable to create %s: %s", prodFileName, err)
	}
	_ = ess.ApplyFileMode(prodDestFile, permRWRWRW)

	cliLog.Infof("Generated 'Dockerfile(s)' at \n\t%s\n\t%s\n", devDestFile, prodDestFile)
	cliLog.Infof("What's next, refer to https://docs.aahframework.org/getting-started-with-docker.html\n")

	return nil
}

func checkAndConfirmOverwrite(c *console.Context, destFile string) bool {
	if ess.IsFileExists(destFile) {
		cliLog.Warnf("File: %s already exists, it will be overwritten.", destFile)
		if c.GlobalBool("y") || c.GlobalBool("yes") {
			fmt.Println("\nWould you like to continue? [y/N]: y")
			return true
		}

		var input string
		for {
			input = readInput(reader, "\nWould you like to continue? [y/N]: ")
			input = strings.ToLower(strings.TrimSpace(input))
			if ess.IsStrEmpty(input) || input == "n" {
				// do not overwrite the file, abort
				fmt.Println()
				cliLog.Warn("Abort!!\n")
				return true
			}

			if input == "y" {
				break
			} else {
				logError("Invalid choice, please provide [Y]es or [N]o")
			}
		}
		fmt.Println()
	}
	return false
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Script Templates
//___________________________________

const aahSystemdScriptTemplate = `# GENERATED BY aah CLI - Feel free to customization it.
# FILE: {{ .FileName }}
# DATE: {{ .CreateDate }}
# DESC: aah application systemd service file

[Unit]
Description={{ .Desc }}
After=network.target

[Service]
#User=aah
#Group=aah
EnvironmentFile=/home/aah/{{ .AppName }}_env_values
ExecStart=/home/aah/{{ .AppName }}/bin/{{ .AppName }} run --envprofile prod
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure

[Install]
WantedBy=multi-user.target
`

const aahDockerDevScriptTemplate = `# GENERATED BY aah CLI - Feel free to customization it.
# FILE: {{ .FileName }}
# DATE: {{ .CreateDate }}
# DESC: aah application {{ .FileName }}

FROM aahframework/aah:{{ .CodeVersion }}

RUN aah --version

ENV AAH_APP_DIR=$GOPATH/src/{{ .AppImportPath }}
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on

RUN mkdir -p $AAH_APP_DIR && \
    cd $AAH_APP_DIR

ADD . $AAH_APP_DIR

WORKDIR $AAH_APP_DIR

EXPOSE 8080
`

const aahDockerProdScriptTemplate = `# GENERATED BY aah CLI - Feel free to customization it.
# FILE: {{ .FileName }}
# DATE: {{ .CreateDate }}
# DESC: aah application {{ .FileName }}, multi stage build - refer to
# https://docs.docker.com/develop/develop-images/multistage-build

#
# Stage 1 : Builder Image
#
FROM aahframework/aah:{{ .CodeVersion }} AS builder
RUN aah --version
ENV AAH_APP_DIR=$GOPATH/src/{{ .AppImportPath }}
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on
RUN mkdir -p $AAH_APP_DIR && \
    cd $AAH_APP_DIR
ADD . $AAH_APP_DIR
WORKDIR $AAH_APP_DIR
RUN aah build --output build/{{ .AppName }}.zip

#
# Stage 2 : Production Image - It creates very small docker image
#
FROM alpine:latest
RUN apk update && \
    apk upgrade && \
    apk --no-cache add ca-certificates
RUN mkdir -p /app/{{ .AppName }}
COPY --from=builder /go/src/{{ .AppImportPath }}/build/{{ .AppName }}.zip /app
RUN cd /app && \
    unzip -q {{ .AppName }}.zip && \
    rm -rf {{ .AppName }}.zip
WORKDIR /app/{{ .AppName }}
CMD ["./bin/{{ .AppName }}", "run", "--envprofile", "prod"]
EXPOSE 8080
`
