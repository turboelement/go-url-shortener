// Package analyze defines a static analyzer that reports:
//   - usage of the built-in panic function anywhere in the code;
//   - calls to log.Fatal, log.Fatalf, log.Fatalln or os.Exit outside
//     func main() of package main.
package analyze

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer is the static analysis checker.
var Analyzer = &analysis.Analyzer{
	Name: "linter",
	Doc:  "reports panic() calls and log.Fatal / os.Exit calls outside func main() of package main",
	Run:  run,
}

// run executes the analysis pass.
func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		pkgName := pass.Pkg.Name()
		inMainFunc := false

		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				// Cheking func main() of package main.
				inMainFunc = pkgName == "main" && x.Name.Name == "main"

			case *ast.CallExpr:
				// Check for built-in panic().
				if ident, ok := x.Fun.(*ast.Ident); ok {
					if ident.Name == "panic" {
						pass.Reportf(x.Pos(), "panic use is not allowed")
					}
				}

				// Check for log.Fatal / os.Exit outside func main() of package main.
				if !inMainFunc || pkgName != "main" {
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						if pkgIdent, ok := sel.X.(*ast.Ident); ok {
							switch pkgIdent.Name {
							case "log":
								if sel.Sel.Name == "Fatal" || sel.Sel.Name == "Fatalf" || sel.Sel.Name == "Fatalln" {
									pass.Reportf(x.Pos(), "call to log.%s outside main function is not allowed", sel.Sel.Name)
								}
							case "os":
								if sel.Sel.Name == "Exit" {
									pass.Reportf(x.Pos(), "call to os.Exit outside main function is not allowed")
								}
							}
						}
					}
				}
			}
			return true
		})
	}
	return nil, nil
}
