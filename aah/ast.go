// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"aahframework.org/essentials"
	"aahframework.org/log"
)

var (
	buildImportCache map[string]string

	// Reference: https://golang.org/pkg/builtin/
	builtInDataTypes = map[string]bool{
		"bool":       true,
		"byte":       true,
		"complex128": true,
		"complex64":  true,
		"error":      true,
		"float32":    true,
		"float64":    true,
		"int":        true,
		"int16":      true,
		"int32":      true,
		"int64":      true,
		"int8":       true,
		"rune":       true,
		"string":     true,
		"uint":       true,
		"uint16":     true,
		"uint32":     true,
		"uint64":     true,
		"uint8":      true,
		"uintptr":    true,
	}
)

type (
	// Program holds all details loaded from the Go source code for given Path.
	program struct {
		Path         string
		Packages     []*packageInfo
		RouteMethods map[string]map[string]uint8
	}

	// PackageInfo holds the single paackge information.
	packageInfo struct {
		Fset       *token.FileSet
		Pkg        *ast.Package
		Types      map[string]*typeInfo
		ImportPath string
		FilePath   string
		Files      []string
	}

	// TypeInfo holds the information about Controller Name, Methods,
	// Embedded types etc.
	typeInfo struct {
		Name          string
		ImportPath    string
		Methods       []*methodInfo
		EmbeddedTypes []*typeInfo
	}

	// MethodInfo holds the information of single method and it's Parameters.
	methodInfo struct {
		Name       string
		StructName string
		Parameters []*parameterInfo
	}

	// ParameterInfo holds the information of single Parameter in the method.
	parameterInfo struct {
		Name       string
		ImportPath string
		Type       *typeExpr
	}

	// TypeExpr holds the information of single parameter data type.
	typeExpr struct {
		Expr         string
		IsBuiltIn    bool
		PackageName  string
		ImportPath   string
		PackageIndex uint8
		Valid        bool
	}
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Global methods
//___________________________________

// LoadProgram method loads the Go source code for the given directory.
func loadProgram(path string, excludes ess.Excludes) (*program, []error) {
	if err := validateInput(path); err != nil {
		return nil, append([]error{}, err)
	}

	prg := &program{
		Path:         path,
		Packages:     []*packageInfo{},
		RouteMethods: map[string]map[string]uint8{},
	}

	var (
		pkgs map[string]*ast.Package
		errs []error
	)

	err := ess.Walk(path, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			errs = append(errs, err)
		}

		if excludes.Match(filepath.Base(srcPath)) {
			if info.IsDir() {
				// excluding directory
				return filepath.SkipDir
			}
			// excluding file
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		if info.IsDir() && ess.IsDirEmpty(srcPath) {
			// skip directory if it's empty
			return filepath.SkipDir
		}

		pfset := token.NewFileSet()
		pkgs, err = parser.ParseDir(pfset, srcPath, func(f os.FileInfo) bool {
			return !f.IsDir() && !excludes.Match(f.Name())
		}, 0)

		if err != nil {
			if errList, ok := err.(scanner.ErrorList); ok {
				// TODO parsing error list
				fmt.Println(errList)
			}

			errs = append(errs, fmt.Errorf("error parsing dir[%s]: %s", srcPath, err))
			return nil
		}

		pkg, err := validatePkgAndGet(pkgs, srcPath)
		if err != nil {
			errs = append(errs, err)
			return nil
		}

		if pkg != nil {
			pkg.Fset = pfset
			pkg.FilePath = srcPath
			pkg.ImportPath = stripGoPath(srcPath)
			prg.Packages = append(prg.Packages, pkg)
		}

		return nil
	})

	if err != nil {
		errs = append(errs, err)
	}

	return prg, errs
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Program methods
//___________________________________

// Process method processes all packages in the program for `Type`,
// `Embedded Type`, `Method`, etc.
func (prg *program) Process() {
	for _, pkgInfo := range prg.Packages {
		pkgInfo.Types = map[string]*typeInfo{}

		// Each source file
		for name, file := range pkgInfo.Pkg.Files {
			pkgInfo.Files = append(pkgInfo.Files, filepath.Base(name))
			var fileImports map[string]string

			// collecting imports
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok {
					if isImportTok(genDecl) {
						fileImports = pkgInfo.processImports(genDecl)
					}
				}
			}

			// collecting types
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok {
					if isTypeTok(genDecl) {
						pkgInfo.processTypes(genDecl, fileImports)
					}
				}
			}

			// collecting methods
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok {
					findMethods(pkgInfo, prg.RouteMethods, funcDecl, fileImports)
				}
			}
		}
	}
}

// FindTypeByEmbeddedType method returns all the typeInfo that has directly or
// indirectly embedded by given type name. Type name must be fully qualified
// type name. E.g.: aahframework.org/aah.Controller
func (prg *program) FindTypeByEmbeddedType(qualifiedTypeName string) []*typeInfo {
	var (
		queue     = []string{qualifiedTypeName}
		processed []string
		result    []*typeInfo
	)

	for len(queue) > 0 {
		typeName := queue[0]
		queue = queue[1:]
		processed = append(processed, typeName)

		// search within all packages in the program
		for _, p := range prg.Packages {
			// search within all struct type in the package
			for _, t := range p.Types {
				// If this one has been processed or is already in queue, then move on.
				if ess.IsSliceContainsString(processed, t.FullyQualifiedName()) ||
					ess.IsSliceContainsString(queue, t.FullyQualifiedName()) {
					continue
				}

				// search through the embedded types to see if the current type is among them.
				for _, et := range t.EmbeddedTypes {
					// If so, add this type's FullyQualifiedName into queue,
					//  and it's typeInfo into result.
					if typeName == et.FullyQualifiedName() {
						queue = append(queue, t.FullyQualifiedName())
						result = append(result, t)
						break
					}
				}
			}
		}
	}

	return result
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// PackageInfo methods
//___________________________________

// Name method return package name
func (p *packageInfo) Name() string {
	return filepath.Base(p.ImportPath)
}

func (p *packageInfo) processTypes(decl *ast.GenDecl, imports map[string]string) {
	spec := decl.Specs[0].(*ast.TypeSpec)
	typeName := spec.Name.Name
	ty := &typeInfo{
		Name:          typeName,
		ImportPath:    p.ImportPath,
		Methods:       []*methodInfo{},
		EmbeddedTypes: []*typeInfo{},
	}

	// struct type
	st, ok := spec.Type.(*ast.StructType)
	if ok {
		// finding embedded type(s) and it's package
		for _, field := range st.Fields.List {
			// If field.Names is set, it's not an embedded type.
			if field.Names != nil && len(field.Names) > 0 {
				continue
			}

			fPkgName, fTypeName := findPkgAndTypeName(field.Type)
			if ess.IsStrEmpty(fTypeName) {
				continue
			}

			// Find the import path for embedded type. If it was referenced without
			// a package name, use the current package import path otherwise
			// look up the package's import path by name.
			var eTypeImportPath string
			if ess.IsStrEmpty(fPkgName) {
				eTypeImportPath = ty.ImportPath
			} else {
				var ok bool
				if eTypeImportPath, ok = imports[fPkgName]; !ok {
					log.Errorf("Unable to find import path for %s.%s", fPkgName, fTypeName)
					continue
				}
			}

			ty.EmbeddedTypes = append(ty.EmbeddedTypes, &typeInfo{Name: fTypeName, ImportPath: eTypeImportPath})
		}
	}

	p.Types[typeName] = ty
}

func (p *packageInfo) processImports(decl *ast.GenDecl) map[string]string {
	imports := map[string]string{}
	for _, dspec := range decl.Specs {
		spec := dspec.(*ast.ImportSpec)
		var pkgAlias string
		if spec.Name != nil {
			if spec.Name.Name == "_" {
				continue
			}

			pkgAlias = spec.Name.Name
		}

		importPath := spec.Path.Value[1 : len(spec.Path.Value)-1]
		if ess.IsStrEmpty(pkgAlias) {
			if alias, found := buildImportCache[importPath]; found {
				pkgAlias = alias
			} else { // build cache
				pkg, err := build.Import(importPath, p.FilePath, 0)
				if err != nil {
					log.Errorf("Unable to find import path: %s", importPath)
					continue
				}
				pkgAlias = pkg.Name
				buildImportCache[importPath] = pkg.Name
			}
		}

		imports[pkgAlias] = importPath
	}

	return imports
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// TypeInfo methods
//___________________________________

// FullyQualifiedName method returns the fully qualified type name.
func (t *typeInfo) FullyQualifiedName() string {
	return fmt.Sprintf("%s.%s", t.ImportPath, t.Name)
}

// PackageName method returns types package name from import path.
func (t *typeInfo) PackageName() string {
	return filepath.Base(t.ImportPath)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// TypeExpr methods
//___________________________________

// Name method returns type name for expression.
func (te *typeExpr) Name() string {
	if te.IsBuiltIn || ess.IsStrEmpty(te.PackageName) {
		return te.Expr
	}

	return fmt.Sprintf("%s%s.%s", te.Expr[:te.PackageIndex], te.PackageName, te.Expr[te.PackageIndex:])
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

func validateInput(path string) error {
	if ess.IsStrEmpty(path) {
		return errors.New("path is required input")
	}

	if !ess.IsFileExists(path) {
		return fmt.Errorf("path is does not exists: %s", path)
	}

	return nil
}

func validatePkgAndGet(pkgs map[string]*ast.Package, path string) (*packageInfo, error) {
	pkgCnt := len(pkgs)

	// no source code found in the directory
	if pkgCnt == 0 {
		return nil, nil
	}

	// not permitted by Go lang spec
	if pkgCnt > 1 {
		var names []string
		for k := range pkgs {
			names = append(names, k)
		}
		return nil, fmt.Errorf("more than one package name [%s] found in single"+
			" directory: %s", strings.Join(names, ", "), path)
	}

	pkg := &packageInfo{}
	for _, v := range pkgs {
		pkg.Pkg = v
	}

	return pkg, nil
}

func isImportTok(decl *ast.GenDecl) bool {
	return token.IMPORT == decl.Tok
}

func isTypeTok(decl *ast.GenDecl) bool {
	return token.TYPE == decl.Tok
}

func stripGoPath(pkgFilePath string) string {
	idx := strings.Index(pkgFilePath, "src")
	return filepath.Clean(pkgFilePath[idx+4:])
}

// findPkgAndTypeName method to find a direct "embedded|sub-type".
// It has an ast.Field as follows:
//   Ident { "type-name" } e.g. UserController
//   SelectorExpr { "package-name", "type-name" } e.g. aah.Controller
// Additionally, that can be wrapped by StarExprs.
func findPkgAndTypeName(fieldType ast.Expr) (string, string) {
	for {
		if starExpr, ok := fieldType.(*ast.StarExpr); ok {
			fieldType = starExpr.X
			continue
		}
		break
	}

	// Embedded type it's in the same package, it's an ast.Ident.
	if ident, ok := fieldType.(*ast.Ident); ok {
		return "", ident.Name
	}

	// Embedded type it's in the different package, it's an ast.SelectorExpr.
	if selectorExpr, ok := fieldType.(*ast.SelectorExpr); ok {
		if pkgIdent, ok := selectorExpr.X.(*ast.Ident); ok {
			return pkgIdent.Name, selectorExpr.Sel.Name
		}
	}

	return "", ""
}

func createImportPaths(types []*typeInfo) map[string]string {
	importPaths := map[string]string{}
	for _, t := range types {
		addPkgAlias(importPaths, t)
	}
	return importPaths
}

func addPkgAlias(importPaths map[string]string, t *typeInfo) {
	if _, found := importPaths[t.ImportPath]; !found {
		cnt := 0
		pkgAlias := t.PackageName()
		for isPkgAliasExists(importPaths, pkgAlias) {
			pkgAlias = fmt.Sprintf("%s%d", t.PackageName(), cnt)
			cnt++
		}
		importPaths[t.ImportPath] = pkgAlias
	}
}

func isPkgAliasExists(importPaths map[string]string, pkgAlias string) bool {
	_, found := importPaths[pkgAlias]
	return found
}

func findMethods(pkg *packageInfo, routeMethods map[string]map[string]uint8, fn *ast.FuncDecl, imports map[string]string) {
	// do not process if -
	// 1. does not have receiver (only methods)
	// 2. method is not exported/public
	// 3. method returns result
	if fn.Recv == nil || !fn.Name.IsExported() ||
		fn.Type.Results != nil {
		return
	}

	var (
		found    bool
		cmethods map[string]uint8
	)

	// if contoller is not configured in routes.conf, no need to process
	controllerName := getName(fn.Recv.List[0].Type)
	if cmethods, found = routeMethods[controllerName]; !found {
		return
	}

	// if action is not configured in routes.conf, no need to process
	actionName := fn.Name.Name
	if _, found = cmethods[actionName]; !found {
		return
	}

	// processed so set to level 2, used for errors later on
	routeMethods[controllerName][actionName] = 2
	method := &methodInfo{Name: actionName, StructName: controllerName, Parameters: []*parameterInfo{}}

	// processing method parameters
	for _, field := range fn.Type.Params.List {
		for _, fieldName := range field.Names {
			typeExpr := parseTypeExpr(pkg.Name(), field.Type)
			if !typeExpr.Valid {
				log.Errorf("Unable to parse parameter '%s' on action '%s.%s', ignoring it", fieldName.Name, controllerName, actionName)
				return
			}

			var importPath string
			if !ess.IsStrEmpty(typeExpr.PackageName) {
				var found bool
				if importPath, found = imports[typeExpr.PackageName]; !found {
					// typeExpr.PackageName = ""
					importPath = pkg.ImportPath
				}
			}

			method.Parameters = append(method.Parameters, &parameterInfo{
				Name:       fieldName.Name,
				ImportPath: importPath,
				Type:       typeExpr,
			})
		}
	}

	ty := pkg.Types[controllerName]
	ty.Methods = append(ty.Methods, method)

	return
}

func getName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}

	if starExpr, ok := expr.(*ast.StarExpr); ok {
		return starExpr.X.(*ast.Ident).Name
	}

	return ""
}

func isBuiltInDataType(typeName string) bool {
	_, found := builtInDataTypes[typeName]
	return found
}

func parseTypeExpr(pkgName string, expr ast.Expr) *typeExpr {
	switch t := expr.(type) {
	case *ast.Ident:
		if isBuiltInDataType(t.Name) {
			return &typeExpr{Expr: t.Name, IsBuiltIn: true, Valid: true}
		}
		return &typeExpr{Expr: t.Name, PackageName: pkgName, Valid: true}
	case *ast.SelectorExpr:
		e := parseTypeExpr(pkgName, t.X)
		return &typeExpr{Expr: t.Sel.Name, PackageName: e.Expr, Valid: e.Valid}
	case *ast.StarExpr:
		e := parseTypeExpr(pkgName, t.X)
		return &typeExpr{Expr: "*" + e.Expr, PackageName: e.PackageName, PackageIndex: e.PackageIndex + uint8(1), Valid: e.Valid}
	case *ast.ArrayType:
		e := parseTypeExpr(pkgName, t.Elt)
		return &typeExpr{Expr: "[]" + e.Expr, PackageName: e.PackageName, PackageIndex: e.PackageIndex + uint8(2), Valid: e.Valid}
	case *ast.Ellipsis:
		e := parseTypeExpr(pkgName, t.Elt)
		return &typeExpr{Expr: "[]" + e.Expr, PackageName: e.PackageName, PackageIndex: e.PackageIndex + uint8(2), Valid: e.Valid}
	}

	return &typeExpr{Valid: false}
}

func init() {
	buildImportCache = map[string]string{}
}
