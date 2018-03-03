// Copyright (c) Jeevanandam M (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
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

	"aahframework.org/essentials.v0"
)

var (
	buildImportCache = map[string]string{}

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

	errInvalidActionParam   = errors.New("aah: invalid action parameter")
	errInterfaceActionParam = errors.New("aah: 'interface{}' is not supported in the action parameter")
	errMapActionParam       = errors.New("aah: 'map' is not supported in the action parameter")
)

type (
	// Program holds all details loaded from the Go source code for given Path.
	program struct {
		Path              string
		Packages          []*packageInfo
		RegisteredActions map[string]map[string]uint8
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
// Package methods
//___________________________________

// LoadProgram method loads the Go source code for the given directory.
func loadProgram(path string, excludes ess.Excludes, registeredActions map[string]map[string]uint8) (*program, []error) {
	if err := validateInput(path); err != nil {
		return nil, append([]error{}, err)
	}

	prg := &program{
		Path:              path,
		Packages:          []*packageInfo{},
		RegisteredActions: registeredActions,
	}

	var (
		pkgs map[string]*ast.Package
		errs []error
	)

	err := ess.Walk(path, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			errs = append(errs, err)
		}

		// Excludes
		if excludes.Match(filepath.Base(srcPath)) {
			if info.IsDir() {
				return filepath.SkipDir
			}

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

		pkg, err := validateAndGetPkg(pkgs, srcPath)
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
			fileImports := make(map[string]string)

			for _, decl := range file.Decls {
				// Processing imports
				pkgInfo.processImports(decl, fileImports)

				// Processing types
				pkgInfo.processTypes(decl, fileImports)

				// Processing methods
				processMethods(pkgInfo, prg.RegisteredActions, decl, fileImports)
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

// CreateImportPaths method returns unique package alias with import path.
func (prg *program) CreateImportPaths(types []*typeInfo) map[string]string {
	importPaths := map[string]string{}
	for _, t := range types {
		createAlias(t.PackageName(), t.ImportPath, importPaths)
		for _, m := range t.Methods {
			for _, p := range m.Parameters {
				if !p.Type.IsBuiltIn {
					createAlias(p.Type.PackageName, p.ImportPath, importPaths)
				}
			}
		}
	}

	return importPaths
}

func createAlias(packageName, importPath string, importPaths map[string]string) {
	importPath = filepath.ToSlash(importPath)
	if _, found := importPaths[importPath]; !found {
		cnt := 0
		pkgAlias := packageName

		for isPkgAliasExists(importPaths, pkgAlias) {
			pkgAlias = fmt.Sprintf("%s%d", packageName, cnt)
			cnt++
		}

		if !ess.IsStrEmpty(pkgAlias) && !ess.IsStrEmpty(importPath) {
			importPaths[importPath] = pkgAlias
		}
	}
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// PackageInfo methods
//___________________________________

// Name method return package name
func (p *packageInfo) Name() string {
	return filepath.Base(p.ImportPath)
}

func (p *packageInfo) processTypes(decl ast.Decl, imports map[string]string) {
	genDecl, ok := decl.(*ast.GenDecl)
	if !ok || !isTypeTok(genDecl) || len(genDecl.Specs) == 0 {
		return
	}

	spec := genDecl.Specs[0].(*ast.TypeSpec)
	st, ok := spec.Type.(*ast.StructType)
	if !ok {
		// Not a struct type
		return
	}

	typeName := spec.Name.Name
	ty := &typeInfo{
		Name:          typeName,
		ImportPath:    filepath.ToSlash(p.ImportPath),
		Methods:       make([]*methodInfo, 0),
		EmbeddedTypes: make([]*typeInfo, 0),
	}

	for _, field := range st.Fields.List {
		// If field.Names is set, it's not an embedded type.
		if field.Names != nil && len(field.Names) > 0 {
			continue
		}

		fPkgName, fTypeName := parseStructFieldExpr(field.Type)
		if ess.IsStrEmpty(fTypeName) {
			continue
		}

		// Find the import path for embedded type. If it was referenced without
		// a package name, use the current package import path otherwise
		// get the import path by package name.
		var eTypeImportPath string
		if ess.IsStrEmpty(fPkgName) {
			eTypeImportPath = ty.ImportPath
		} else {
			var found bool
			if eTypeImportPath, found = imports[fPkgName]; !found {
				logErrorf("AST: Unable to find import path for %s.%s", fPkgName, fTypeName)
				continue
			}
		}

		ty.EmbeddedTypes = append(ty.EmbeddedTypes, &typeInfo{Name: fTypeName, ImportPath: eTypeImportPath})
	}

	p.Types[typeName] = ty
}

func (p *packageInfo) processImports(decl ast.Decl, imports map[string]string) {
	genDecl, ok := decl.(*ast.GenDecl)
	if !ok || !isImportTok(genDecl) {
		return
	}

	for _, dspec := range genDecl.Specs {
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
					logErrorf("AST: Unable to find import path: %s", importPath)
					continue
				}
				pkgAlias = pkg.Name
				buildImportCache[importPath] = pkg.Name
			}
		}

		imports[pkgAlias] = importPath
	}
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

func validateAndGetPkg(pkgs map[string]*ast.Package, path string) (*packageInfo, error) {
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

func isPkgAliasExists(importPaths map[string]string, pkgAlias string) bool {
	_, found := importPaths[pkgAlias]
	return found
}

func processMethods(pkg *packageInfo, routeMethods map[string]map[string]uint8, decl ast.Decl, imports map[string]string) {
	fn, ok := decl.(*ast.FuncDecl)

	// Do not process if these met:
	// 		1. does not have receiver, it means package function/method
	// 		2. method is not exported
	// 		3. method returns result
	if !ok || fn.Recv == nil || !fn.Name.IsExported() ||
		fn.Type.Results != nil {
		return
	}

	actionName := fn.Name.Name
	if isInterceptorActioName(actionName) {
		return
	}

	controllerName := getName(fn.Recv.List[0].Type)
	method := &methodInfo{Name: actionName, StructName: controllerName, Parameters: []*parameterInfo{}}

	// processed so set to level 2, used to display unimplemented action details
	// TODO for controller check too
	for k, v := range routeMethods {
		if strings.HasSuffix(k, controllerName) {
			if _, found := v[actionName]; found {
				v[actionName] = 2
			}
		}
	}

	// processing method parameters
	for _, field := range fn.Type.Params.List {
		for _, fieldName := range field.Names {
			te, err := parseParamFieldExpr(pkg.Name(), field.Type)
			if err != nil {
				logErrorf("AST: %s, please fix the parameter '%s' on action '%s.%s'; "+
					"otherwise your action may not work properly", err, fieldName.Name, controllerName, actionName)
				continue
			}

			var importPath string
			if !ess.IsStrEmpty(te.PackageName) {
				var found bool
				if importPath, found = imports[te.PackageName]; !found {
					importPath = pkg.ImportPath
				}
			}

			method.Parameters = append(method.Parameters, &parameterInfo{
				Name:       fieldName.Name,
				ImportPath: importPath,
				Type:       te,
			})
		}
	}

	if ty := pkg.Types[controllerName]; ty == nil {
		pos := pkg.Fset.Position(decl.Pos())
		filename := stripGoPath(pos.Filename)
		logErrorf("AST: Method '%s' has incorrect struct recevier '%s' on file [%s] at line #%d",
			actionName, controllerName, filename, pos.Line)
	} else {
		ty.Methods = append(ty.Methods, method)
	}
}

func isInterceptorActioName(actionName string) bool {
	return (strings.HasPrefix(actionName, "Before") || strings.HasPrefix(actionName, "After") ||
		strings.HasPrefix(actionName, "Panic") || strings.HasPrefix(actionName, "Finally"))
}

func getName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return getName(t.X)
	case *ast.StarExpr:
		return getName(t.X)
	default:
		return ""
	}
}

func isBuiltInDataType(typeName string) bool {
	_, found := builtInDataTypes[typeName]
	return found
}

// parseStructFieldExpr method to find a direct "embedded|sub-type".
// Struct ast.Field as follows:
//   Ident { "type-name" } e.g. UserController
//   SelectorExpr { "package-name", "type-name" } e.g. aah.Controller
//   StarExpr { "*", "package-name", "type-name"} e.g. *aah.Controller
func parseStructFieldExpr(fieldType ast.Expr) (string, string) {
	for {
		if starExpr, ok := fieldType.(*ast.StarExpr); ok {
			fieldType = starExpr.X
			continue
		}
		break
	}

	// type it's in the same package, it's an ast.Ident.
	if ident, ok := fieldType.(*ast.Ident); ok {
		return "", ident.Name
	}

	// type it's in the different package, it's an ast.SelectorExpr.
	if selectorExpr, ok := fieldType.(*ast.SelectorExpr); ok {
		if pkgIdent, ok := selectorExpr.X.(*ast.Ident); ok {
			return pkgIdent.Name, selectorExpr.Sel.Name
		}
	}

	return "", ""
}

func parseParamFieldExpr(pkgName string, expr ast.Expr) (*typeExpr, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		if isBuiltInDataType(t.Name) {
			return &typeExpr{Expr: t.Name, IsBuiltIn: true}, nil
		}
		return &typeExpr{Expr: t.Name, PackageName: pkgName}, nil
	case *ast.SelectorExpr:
		e, err := parseParamFieldExpr(pkgName, t.X)
		return &typeExpr{Expr: t.Sel.Name, PackageName: e.Expr}, err
	case *ast.StarExpr:
		e, err := parseParamFieldExpr(pkgName, t.X)
		return &typeExpr{Expr: "*" + e.Expr, PackageName: e.PackageName, PackageIndex: e.PackageIndex + uint8(1)}, err
	case *ast.ArrayType:
		e, err := parseParamFieldExpr(pkgName, t.Elt)
		return &typeExpr{Expr: "[]" + e.Expr, PackageName: e.PackageName, PackageIndex: e.PackageIndex + uint8(2)}, err
	case *ast.Ellipsis:
		e, err := parseParamFieldExpr(pkgName, t.Elt)
		return &typeExpr{Expr: "[]" + e.Expr, PackageName: e.PackageName, PackageIndex: e.PackageIndex + uint8(2)}, err
	case *ast.InterfaceType:
		return nil, errInterfaceActionParam
	case *ast.MapType:
		return nil, errMapActionParam
	}

	return nil, errInvalidActionParam
}
