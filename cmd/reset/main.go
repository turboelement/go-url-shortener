package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// resetConfig stores parsed structure info for generation.
type resetConfig struct {
	PackageName string
	Methods     []resetMethod
}

type resetMethod struct {
	StructName  string
	ReceiverVar string
	Fields      []fieldInfo
}

type fieldInfo struct {
	Name        string
	Kind        fieldKind
	IsStruct    bool
	IsPtr       bool
	IsMap       bool
	IsSlice     bool
	IsPtrStruct bool
}

const generateResetDirective = "generate:reset"

type fieldKind int

const (
	kindPrimitive fieldKind = iota
	kindString
	kindBool
	kindSlice
	kindMap
	kindStruct
	kindPtr
	kindOther
)

// builtin types
var primitiveTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
	"complex64": true, "complex128": true,
	"byte": true, "rune": true, "uintptr": true,
}

var stringTypes = map[string]bool{
	"string": true,
}

var boolTypes = map[string]bool{
	"bool": true,
}

// pkgTypeInfo stores info about declared types.
type pkgTypeInfo struct {
	structNames     map[string]bool
	aliasUnderlying map[string]string
}

// collectPkgTypes parses all .go files in the directory.
func collectPkgTypes(fset *token.FileSet, dir string) (*pkgTypeInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	info := &pkgTypeInfo{
		structNames:     make(map[string]bool),
		aliasUnderlying: make(map[string]string),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		filename := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				typeName := typeSpec.Name.Name

				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					info.structNames[typeName] = true
				} else if ident, ok := typeSpec.Type.(*ast.Ident); ok {
					// type alias like `type Action string`
					info.aliasUnderlying[typeName] = ident.Name
				}
			}
		}
	}

	return info, nil
}

// resolveUnderlyingType resolves type name to underlying primitive type.
func resolveUnderlyingType(typeName string, info *pkgTypeInfo) (string, bool) {
	visited := map[string]bool{typeName: true}
	current := typeName

	for {
		if primitiveTypes[current] || stringTypes[current] || boolTypes[current] {
			return current, true
		}
		underlying, ok := info.aliasUnderlying[current]
		if !ok || visited[underlying] {
			return current, false
		}
		visited[underlying] = true
		current = underlying
	}
}

// getFieldInfo extracts field metadata from ast.Field.
func getFieldInfo(field *ast.Field, info *pkgTypeInfo) []fieldInfo {
	if len(field.Names) == 0 {
		return nil // embedded field, skip
	}

	var results []fieldInfo
	typeExpr := field.Type

	// Handle pointers
	isPtr := false
	if starExpr, ok := typeExpr.(*ast.StarExpr); ok {
		isPtr = true
		typeExpr = starExpr.X
	}

	// Handle slices
	if _, ok := typeExpr.(*ast.ArrayType); ok {
		for _, name := range field.Names {
			results = append(results, fieldInfo{
				Name:    name.Name,
				Kind:    kindSlice,
				IsPtr:   isPtr,
				IsSlice: true,
			})
		}
		return results
	}

	// Handle maps
	if _, ok := typeExpr.(*ast.MapType); ok {
		for _, name := range field.Names {
			results = append(results, fieldInfo{
				Name:  name.Name,
				Kind:  kindMap,
				IsPtr: isPtr,
				IsMap: true,
			})
		}
		return results
	}

	// Handle inline anonymous struct
	if _, ok := typeExpr.(*ast.StructType); ok {
		for _, name := range field.Names {
			results = append(results, fieldInfo{
				Name:     name.Name,
				Kind:     kindStruct,
				IsStruct: true,
				IsPtr:    isPtr,
			})
		}
		return results
	}

	// Handle named types (Ident)
	if ident, ok := typeExpr.(*ast.Ident); ok {
		typeName := ident.Name
		underlyingType, isKnown := resolveUnderlyingType(typeName, info)

		if isKnown {
			// It's a primitive/string/bool or alias
			kind := kindPrimitive
			if stringTypes[underlyingType] {
				kind = kindString
			} else if boolTypes[underlyingType] {
				kind = kindBool
			}
			for _, name := range field.Names {
				results = append(results, fieldInfo{
					Name:  name.Name,
					Kind:  kind,
					IsPtr: isPtr,
				})
			}
			return results
		}

		// It's not a known primitive — check if it's a struct type
		if info.structNames[typeName] {
			if isPtr {
				for _, name := range field.Names {
					results = append(results, fieldInfo{
						Name:        name.Name,
						Kind:        kindPtr,
						IsPtr:       true,
						IsPtrStruct: true,
					})
				}
			} else {
				for _, name := range field.Names {
					results = append(results, fieldInfo{
						Name:     name.Name,
						Kind:     kindStruct,
						IsStruct: true,
					})
				}
			}
			return results
		}

		// Unknown named type — treat as other
		for _, name := range field.Names {
			results = append(results, fieldInfo{
				Name:  name.Name,
				Kind:  kindOther,
				IsPtr: isPtr,
			})
		}
		return results
	}

	// Handle selector expressions (e.g., os.File, time.Time)
	if sel, ok := typeExpr.(*ast.SelectorExpr); ok {
		_ = sel
		for _, name := range field.Names {
			results = append(results, fieldInfo{
				Name:  name.Name,
				Kind:  kindOther,
				IsPtr: isPtr,
			})
		}
		return results
	}

	// Fallback for any other type expression
	for _, name := range field.Names {
		results = append(results, fieldInfo{
			Name:  name.Name,
			Kind:  kindOther,
			IsPtr: isPtr,
		})
	}
	return results
}

// hasResetComment checks if the field's doc group contains "generate:reset".
// Also handles the case where comment is on the line before type spec.
func hasGenerateReset(genDecl *ast.GenDecl) bool {
	if genDecl.Doc != nil {
		for _, comment := range genDecl.Doc.List {
			if strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")) == generateResetDirective {
				return true
			}
		}
	}
	return false
}

// receiverName builds a short variable name from capital letters of struct name.
// e.g. Config → c, ResetableStruct → rs, URLEntry → urle
func receiverName(structName string) string {
	var caps []rune
	for _, r := range structName {
		if r >= 'A' && r <= 'Z' {
			caps = append(caps, r)
		}
	}
	if len(caps) == 0 {
		return "rs"
	}
	return strings.ToLower(string(caps))
}

// processFile parses single .go file and returns structs with "generate:reset".
func processFile(fset *token.FileSet, filename string, info *pkgTypeInfo) ([]resetMethod, error) {
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	var methods []resetMethod

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		if !hasGenerateReset(genDecl) {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			var fields []fieldInfo
			for _, field := range structType.Fields.List {
				fieldInfos := getFieldInfo(field, info)
				fields = append(fields, fieldInfos...)
			}

			methods = append(methods, resetMethod{
				StructName:  typeSpec.Name.Name,
				ReceiverVar: receiverName(typeSpec.Name.Name),
				Fields:      fields,
			})
		}
	}

	return methods, nil
}

// processPackage processes all .go files (except test files and reset.gen.go) in the directory.
func processPackage(dir string) ([]resetMethod, error) {
	fset := token.NewFileSet()

	// First pass: collect type information for the package.
	info, err := collectPkgTypes(fset, dir)
	if err != nil {
		return nil, fmt.Errorf("collect types in %s: %w", dir, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var allMethods []resetMethod
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "reset.gen.go" {
			continue
		}

		filename := filepath.Join(dir, name)
		methods, err := processFile(fset, filename, info)
		if err != nil {
			return nil, err
		}
		allMethods = append(allMethods, methods...)
	}
	return allMethods, nil
}

const resetTmpl = `// Code generated by ` + generateResetDirective + `; DO NOT EDIT.

package {{ .PackageName }}
{{ range .Methods }}{{ $r := .ReceiverVar }}
func ({{ $r }} *{{ .StructName }}) Reset() {
	if {{ $r }} == nil {
		return
	}
{{- range .Fields }}
{{- if .IsPtr }}
{{- if .IsPtrStruct }}
	if {{ $r }}.{{ .Name }} != nil {
		if resetter, ok := interface{}({{ $r }}.{{ .Name }}).(interface{ Reset() }); ok {
			resetter.Reset()
		}
	}
{{- else if .IsSlice }}
	if {{ $r }}.{{ .Name }} != nil {
		{{ $r }}.{{ .Name }} = {{ $r }}.{{ .Name }}[:0]
	}
{{- else if .IsMap }}
	if {{ $r }}.{{ .Name }} != nil {
		clear({{ $r }}.{{ .Name }})
	}
{{- else if eq .Kind 0 }}
	if {{ $r }}.{{ .Name }} != nil {
		*{{ $r }}.{{ .Name }} = 0
	}
{{- else if eq .Kind 1 }}
	if {{ $r }}.{{ .Name }} != nil {
		*{{ $r }}.{{ .Name }} = ""
	}
{{- else if eq .Kind 2 }}
	if {{ $r }}.{{ .Name }} != nil {
		*{{ $r }}.{{ .Name }} = false
	}
{{- end }}
{{- else if .IsStruct }}
	if resetter, ok := interface{}(&{{ $r }}.{{ .Name }}).(interface{ Reset() }); ok {
		resetter.Reset()
	}
{{- else if .IsSlice }}
	{{ $r }}.{{ .Name }} = {{ $r }}.{{ .Name }}[:0]
{{- else if .IsMap }}
	clear({{ $r }}.{{ .Name }})
{{- else if eq .Kind 0 }}
	{{ $r }}.{{ .Name }} = 0
{{- else if eq .Kind 1 }}
	{{ $r }}.{{ .Name }} = ""
{{- else if eq .Kind 2 }}
	{{ $r }}.{{ .Name }} = false
{{- end }}
{{- end }}
}
{{ end }}
`

func generateResetFile(pkg string, methods []resetMethod) (string, error) {
	tmpl, err := template.New("reset").Parse(resetTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	cfg := resetConfig{
		PackageName: pkg,
		Methods:     methods,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func main() {
	// Root directory: either the first argument or current working directory.
	rootDir := "."
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root path: %v\n", err)
		os.Exit(1)
	}

	type pkgResult struct {
		Dir     string
		PkgName string
		Methods []resetMethod
	}

	var results []pkgResult

	err = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		// Skip hidden directories and vendored directories.
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") && base != "." {
			return filepath.SkipDir
		}
		if base == "vendor" || base == "node_modules" || base == "testdata" {
			return filepath.SkipDir
		}
		if strings.Contains(path, string(filepath.Separator)+"mocks") {
			return filepath.SkipDir
		}

		// Check if this directory has any Go files.
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		hasGoFiles := false
		var pkgName string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				hasGoFiles = true
				// Parse just to get the package name.
				fset := token.NewFileSet()
				f, err := parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, parser.PackageClauseOnly)
				if err == nil && f.Name != nil {
					pkgName = f.Name.Name
					break
				}
			}
		}

		if !hasGoFiles || pkgName == "" {
			return nil
		}
		if pkgName == "main" {
			// Skip main packages.
			return nil
		}

		methods, err := processPackage(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error processing %s: %v\n", path, err)
			return nil
		}

		if len(methods) > 0 {
			results = append(results, pkgResult{
				Dir:     path,
				PkgName: pkgName,
				Methods: methods,
			})
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("No structs with // %s found.\n", generateResetDirective)
		return
	}

	for _, res := range results {
		content, err := generateResetFile(res.PkgName, res.Methods)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating reset code for %s: %v\n", res.Dir, err)
			continue
		}

		outputFile := filepath.Join(res.Dir, "reset.gen.go")
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputFile, err)
			continue
		}
		fmt.Printf("Generated %s (%d struct(s))\n", outputFile, len(res.Methods))
	}
}
