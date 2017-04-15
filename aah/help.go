// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

var helpCmd = &command{
	Name:      "help",
	UsageLine: "aah help [command]",
	ArgsCount: 1,
	Short:     "to learn about aah command",
	Long: `
'aah help' displays the command usage.

Go to https://docs.aahframework.org/aah-cli-tool.html to learn more.

Example:
    aah help

    aah help [command-name]
`,
	Run: func(args []string) {
		if len(args) == 0 {
			displayUsage()
			return
		}

		cmd, err := subCmds.Find(args[0])
		if err != nil {
			commandNotFound(args[0])
		}

		cmd.Usage()
	},
}
