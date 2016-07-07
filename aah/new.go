package main

import "fmt"

var cmdNew = &Command{
	Name:      "new",
	UsageLine: "aah new importPath [profile]",
	Short:     "create a new aah 'web' or 'api' application",
	Long: `
Command 'new' helps you to create & quickly start with aah application.

It puts all of the files based on [profile] in the given import path, taking the final element in
the path to be the app name, no worries you can change it later in the configuration.

Parameter(s):
importPath      mandatory   for e.g: github.com/user/appname
profile         optional    web or api, defaluts to web

For example:
    aah new github.com/user/appname

    aah new github.com/user/appname api
`,
}

func init() {
	cmdNew.Run = newCommand
}

func newCommand(args []string) {
	fmt.Println("called new command")
}
