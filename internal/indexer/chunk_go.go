package indexer

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// ChunkGo extracts semantic chunks from Go source code.
// Each chunk corresponds to a function or method and includes
// its preceding comment and file path to improve embedding quality.
func ChunkGo(path, src string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return ChunkText(path, src)
	}

	var chunks []string

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		body := src[start:end]

		if fn.Doc != nil {
			chunks = append(chunks, formatChunk(path, fn.Doc.Text(), body))
			continue
		}
		chunks = append(chunks, formatChunk(path, body))
	}

	return chunks
}
