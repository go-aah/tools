// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"testing"

	ess "aahframework.org/essentials.v0"
)

func TestLoad(t *testing.T) {
	excludes := ess.Excludes{
		"*_test.go",
		".*",
		"*.bak",
		"*.tmp",
		"tmp",
		"routes",
	}

	prg, errs := loadProgram("/Users/jeeva/pscm/go-home/src/bitbucket.org/getrightcare/api/app/controllers", excludes, nil)

	fmt.Println(errs)

	// for _, p := range prg.Packages {
	// 	fmt.Println("Name:", p.Name())
	// }

	prg.Process()
	fmt.Println("===============================")

	// cntPkg := prg.FindPackage("controllers")
	// fmt.Println("cntPkg.Files:", cntPkg.Files)
	// fmt.Println("Controllers Pkg types:", cntPkg.Types, len(cntPkg.Types))

	etypes := prg.FindTypeByEmbeddedType("aahframework.org/aah.v0.Controller")
	fmt.Println("Embedded:", etypes)
	for _, v := range etypes {
		fmt.Println(v)
	}

	fmt.Println("All packages:")
	for _, p := range prg.Packages {
		fmt.Println("Package:", p.ImportPath)
		for _, t := range p.Types {
			fmt.Println("\tType:", t.FullyQualifiedName())
			for _, m := range t.Methods {
				fmt.Println("\t\tMethod:", m)
			}
			for _, et := range t.EmbeddedTypes {
				fmt.Println("\t\tEmbeddedType:", et.FullyQualifiedName())
			}
		}
	}

	// fmt.Println("All packages:")
	// for _, p := range prg.Packages {
	// 	fmt.Println("Package:", p.ImportPath)
	// 	for _, t := range p.Types {
	// 		fmt.Println("\tType:", t.FullyQualifiedName())
	// 		for _, m := range t.Methods {
	// 			fmt.Println("\t\tMethod:", m)
	// 			for _, pr := range m.Parameters {
	// 				fmt.Println("\t\t\tParameter:", pr, pr.Type.Name())
	// 			}
	// 		}
	// 	}
	// }

	// fmt.Println(cntPkg.Path, cntPkg.FilePath, cntPkg.Types)

	// prg.Fset.Iterate(func(f *token.File) bool {
	// 	fmt.Println("FileName:", f.Name())
	// 	return true
	// })
}
