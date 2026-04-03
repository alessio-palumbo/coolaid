package indexer

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// ChunkGo extracts semantic chunks from Go source code.
// Each chunk corresponds to a function or method and includes
// its preceding comment and file path to improve embedding quality.
func ChunkGo(path string, src []byte) []Chunk {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return ChunkText(path, src)
	}

	var chunks []Chunk

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		startOffset := fset.Position(fn.Pos()).Offset
		if fn.Doc != nil {
			startOffset = fset.Position(fn.Doc.Pos()).Offset
		}
		endOffset := fset.Position(fn.End()).Offset

		startLine := fset.Position(fn.Pos()).Line
		endLine := fset.Position(fn.End()).Line

		chunks = append(chunks, NewChunk(
			startLine, endLine, fn,
			formatChunk(path, startLine, endLine, fn, src[startOffset:endOffset]),
		))
	}

	return chunks
}
