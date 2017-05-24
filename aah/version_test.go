// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

// Note: aah CLI tool test case approach is similar to
// https://github.com/golang/go/blob/master/src/cmd/go/go_test.go.
// Ensure implementation works as expected but not on the code coverage.

package main

import (
	"fmt"
	"os"
	"testing"

	"aahframework.org/test.v0/assert"
)

func TestVersion(t *testing.T) {
	exit = func(code int) {}
	os.Args = []string{"aah", "version", "-all"}
	main()

	*allFlag = false
	fatal = func(v ...interface{}) {
		assert.Equal(t, "bad flag syntax: ---all", fmt.Sprint(v...))
	}
	versionRun([]string{"---all"})
}
