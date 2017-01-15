// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
	"aahframework.org/test"
)

var cmdVersion = &command{
	Name:      "version",
	UsageLine: "aah version [all]",
	ArgsCount: 1,
	Short:     "print aah framework version and Go version",
	Long: `
  Prints the aah framework, modules version and Go version.

  For example:

    aah version
		aah version all
`,
}

func versionRun(args []string) {
	fmt.Printf("Version Info:\n")
	printVersion("aah framework", aah.Version)

	if len(args) > 0 {
		if args[0] == "all" {
			printVersion("config", config.Version)
			printVersion("log", log.Version)
			printVersion("essentials", ess.Version)
			printVersion("test", test.Version)
		}
	}

	printVersion(fmt.Sprintf("go[%s/%s]", runtime.GOOS, runtime.GOARCH), runtime.Version()[2:])
	fmt.Println()
}

func printVersion(name, version string) {
	fmt.Printf("\t%-17s v%s\n", name, version)
}

func init() {
	cmdVersion.Run = versionRun
}
