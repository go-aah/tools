// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"runtime"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/ahttp.v0"
	"aahframework.org/aruntime.v0"
	"aahframework.org/atemplate.v0"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/i18n.v0"
	"aahframework.org/log.v0"
	"aahframework.org/pool.v0"
	"aahframework.org/router.v0"
	"aahframework.org/test.v0"
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
	if err := versionCmdFlags.Parse(args); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Version Info:\n")
	printVersion("aah framework", aah.Version)
	printVersion("aah cli tool", Version)

	if *allFlag {
		printVersion("ahttp", ahttp.Version)
		printVersion("atemplate", atemplate.Version)
		printVersion("aruntime", aruntime.Version)
		printVersion("router", router.Version)
		printVersion("i18n", i18n.Version)
		printVersion("config", config.Version)
		printVersion("config", config.Version)
		printVersion("log", log.Version)
		printVersion("essentials", ess.Version)
		printVersion("pool", pool.Version)
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
