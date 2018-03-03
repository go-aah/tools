// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"aahframework.org/aah.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"

	"gopkg.in/urfave/cli.v1"
)

var generateCmd = cli.Command{
	Name:    "generate",
	Aliases: []string{"g"},
	Usage:   "Generates boilerplate code, configurations and complement scripts",
	Description: `Generate command increases productivity and helps developer on tedious tasks during application development.
  It generates boilerplate code, configuration files and complement scripts, etc.

	To know more about available 'generate' sub commands:
		aah g h
		aah generate help

	To know more about individual sub command details:
		aah g h s
		aah generate help script
`,
	Subcommands: []cli.Command{
		cli.Command{
			Name:    "script",
			Aliases: []string{"s"},
			Usage:   "Generates complement scripts such as systemd, dockerize, etc.",
			Description: `Generates complement scripts such as systemd, dockerize, etc.

	Example of script command:
		aah g s -n systemd -i github.com/user/appname
		aah generate script --name systemd --importpath github.com/user/appname
			`,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "n, name",
					Usage: "Provide script name such as 'systemd', 'docker', etc",
				},
				cli.StringFlag{
					Name:  "i, importpath",
					Usage: "Import path of aah application",
				},
			},
			Action: generateScriptsAction,
		},
	},
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Generate Subcommand - Script
//___________________________________

func generateScriptsAction(c *cli.Context) error {
	scriptName := strings.TrimSpace(firstNonEmpty(c.String("n"), c.String("name")))
	if ess.IsStrEmpty(scriptName) {
		_ = cli.ShowSubcommandHelp(c)
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

func generateSystemdScript(c *cli.Context) error {
	importPath := getAppImportPath(c)

	aah.Init(importPath)
	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)

	cliLog.Infof("Loaded aah project file: %s\n", filepath.Join(aah.AppBaseDir(), aahProjectIdentifier))

	fileName := fmt.Sprintf("%s.service", aah.AppName())
	destFile := filepath.Join(aah.AppBaseDir(), fileName)
	if checkAndConfirmOverwrite(destFile) {
		return nil
	}

	data := map[string]interface{}{
		"AppName":    aah.AppName(),
		"FileName":   fileName,
		"CreateDate": time.Now().Format(time.RFC1123Z),
		"Desc":       fmt.Sprintf("%s application", aah.AppName()),
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

func generateDockerScript(c *cli.Context) error {
	importPath := getAppImportPath(c)

	aah.Init(importPath)
	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)

	cliLog.Infof("Loaded aah project file: %s\n", filepath.Join(aah.AppBaseDir(), aahProjectIdentifier))

	fileName := "Dockerfile"
	destFile := filepath.Join(aah.AppBaseDir(), fileName)
	if checkAndConfirmOverwrite(destFile) {
		return nil
	}

	codeVersion := aah.Version
	if strings.HasSuffix(codeVersion, "-edge") {
		codeVersion = "edge"
	}
	data := map[string]interface{}{
		"AppName":       aah.AppName(),
		"AppImportPath": aah.AppImportPath(),
		"FileName":      fileName,
		"CreateDate":    time.Now().Format(time.RFC1123Z),
		"CodeVersion":   codeVersion,
	}

	buf := &bytes.Buffer{}
	if err := renderTmpl(buf, aahDockerScriptTemplate, data); err != nil {
		return fmt.Errorf("Unable to create Dockerfile: %s", err)
	}
	if err := ioutil.WriteFile(destFile, buf.Bytes(), permRWXRXRX); err != nil {
		return fmt.Errorf("Unable to create Dockerfile: %s", err)
	}

	cliLog.Infof("Generated 'Dockerfile' at '%s'\n", destFile)
	cliLog.Infof("What's next, refer to https://docs.aahframework.org/getting-started-with-docker.html\n")

	return nil
}

func checkAndConfirmOverwrite(destFile string) bool {
	if ess.IsFileExists(destFile) {
		cliLog.Warnf("File: %s already exists, it will be overwritten.", destFile)
		var input string
		for {
			input = readInput(reader, "\nWould you like to continue [Y]es or [N]o, default is 'N'? ")
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

const aahSystemdScriptTemplate = `// GENERATED BY aah CLI TOOL - Feel free to customization it.
// FILE: {{ .FileName }}
// DATE: {{ .CreateDate }}
// DESC: aah application systemd service file

[Unit]
Description={{ .Desc }}
After=network.target

[Service]
EnvironmentFile=/home/aah/{{ .AppName }}_env_values
User=aah
Group=aah
Type=forking
ExecStart=/home/aah/{{ .AppName }}/aah.sh start
ExecStop=/home/aah/{{ .AppName }}/aah.sh stop
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure

[Install]
WantedBy=multi-user.target
`

const aahDockerScriptTemplate = `// GENERATED BY aah CLI TOOL - Feel free to customization it.
// FILE: {{ .FileName }}
// DATE: {{ .CreateDate }}
// DESC: aah application Dockerfile

FROM aahframework/aah:{{ .CodeVersion }}

RUN aah --version

ENV AAH_APP_DIR=$GOPATH/src/{{ .AppImportPath }}

RUN mkdir -p $AAH_APP_DIR

RUN cd $AAH_APP_DIR

ADD . $AAH_APP_DIR

WORKDIR $AAH_APP_DIR

EXPOSE 8080
`
