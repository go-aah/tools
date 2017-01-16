// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"runtime"

	"aahframework.org/aah"
	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
	"aahframework.org/test"
)

var (
	versionCmdFlags = flag.NewFlagSet("version", flag.ExitOnError)
	allFlag         = versionCmdFlags.Bool("all", false, "Display aah framework, modules version and go version")
	versionCmd      = &command{
		Name:      "version",
		UsageLine: "aah version [-all]",
		Flags:     versionCmdFlags,
		ArgsCount: 1,
		Short:     "print aah framework version and go version",
		Long: `
	  Prints the aah framework, modules version and go version.

	  For example:

	    aah version
			aah version -all
	`,
	}
)

func versionRun(args []string) {
	versionCmdFlags.Parse(args)

	fmt.Printf("Version Info:\n")
	printVersion("aah framework", aah.Version)

	if *allFlag {
		printVersion("config", config.Version)
		printVersion("log", log.Version)
		printVersion("essentials", ess.Version)
		printVersion("test", test.Version)
	}

	printVersion(fmt.Sprintf("go[%s/%s]", runtime.GOOS, runtime.GOARCH), runtime.Version()[2:])
	fmt.Println()
}

func printVersion(name, version string) {
	fmt.Printf("\t%-17s v%s\n", name, version)
}

func init() {
	versionCmd.Run = versionRun
}
