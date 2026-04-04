package query

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// goBuiltins is a simple map of common go built-in that should
// be excluded from the returned signals.
var goBuiltins = map[string]struct{}{
	"len": {}, "cap": {}, "make": {}, "new": {},
	"append": {}, "delete": {}, "copy": {},
}

// ExtractSignals returns a signal-rich textual representation of a file
// used for embedding and semantic search.
//
// It extracts meaningful identifiers (e.g. functions, method calls, imports)
// using language-specific strategies where available (e.g. Go AST), and falls
// back to heuristic text extraction for other file types.
//
// The returned string is optimized for retrieval quality, not for display or
// direct use in LLM prompts.
func ExtractSignals(path string, content []byte, includeLocal bool) string {
	switch filepath.Ext(path) {
	case ".go":
		return extractGoSignals(content, includeLocal)
	default:
		return extractTextSignals(content)
	}
}

// extractGoSignals extracts meaningful identifiers (function calls and definitions)
// from Go source code using the AST.
func extractGoSignals(src []byte, includeLocal bool) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return string(src) // fallback
	}

	var out []string
	add := dedupingAdder(&out)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// function definition
			if includeLocal {
				add(node.Name.Name)
			}
		case *ast.CallExpr:
			switch fun := node.Fun.(type) {
			// identifier
			case *ast.Ident:
				if _, isBuiltin := goBuiltins[fun.Name]; !isBuiltin {
					add(fun.Name)
				}
			// method call
			case *ast.SelectorExpr:
				if pkgIdent, ok := fun.X.(*ast.Ident); ok {
					add(pkgIdent.Name + "." + fun.Sel.Name)
				} else if includeLocal {
					add(fun.Sel.Name)
				}
			}
		case *ast.ImportSpec:
			if node.Path != nil {
				path := strings.Trim(node.Path.Value, `"`)
				add(path)
			}
		}

		return true
	})

	return strings.Join(out, "\n")
}

// extractTextSignals extracts meaningful tokens from non-Go files.
// It attempts to capture identifiers and function-like names.
func extractTextSignals(src []byte) string {
	indexes := reIdentifier.FindAllIndex(src, -1)

	out := make([]string, 0, len(indexes))
	add := dedupingAdder(&out)
	for _, idx := range indexes {
		add(string(src[idx[0]:idx[1]]))
	}
	return strings.Join(out, "\n")
}

func dedupingAdder(out *[]string) func(name string) {
	seen := make(map[string]struct{})
	return func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		*out = append(*out, name)
	}
}
