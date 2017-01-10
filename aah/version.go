// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"

	"aahframework.org/aah"
)

var cmdVersion = &Command{
	Name:      "version",
	UsageLine: "aah version",
	Short:     "print aah framework version and Go version",
	Long: `
  Command 'version' prints the aah framework and Go version.

  For example:

      aah version
`,
}

func versionRun(args []string) {
	fmt.Printf("Version Info:")
	fmt.Printf("\n   aah framework v%v", aah.Version)
	fmt.Printf("\n   %s %s/%s\n\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func init() {
	cmdVersion.Run = versionRun
}
