package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
)

// Chunk represents the universal contract defined in the Onfly SDD
type Chunk struct {
	FilePath string            `json:"file_path"`
	Type     string            `json:"type"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
}

// fatalError ensures errors are written to stderr so the Onfly core
// can log them properly without breaking the stdout JSON pipeline.
func fatalError(err error) {
	fmt.Fprintf(os.Stderr, "onfly-go-parser error: %v\n", err)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fatalError(fmt.Errorf("missing file path argument"))
	}
	filePath := os.Args[1]
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatalError(fmt.Errorf("failed to read stdin: %w", err))
	}
	chunks, err := parse(filePath, src)
	if err != nil {
		fatalError(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(chunks); err != nil {
		fatalError(fmt.Errorf("failed to encode json: %w", err))
	}
}

func parse(filePath string, src []byte) ([]Chunk, error) {
	fset := token.NewFileSet()
	// parser.ParseComments is critical here so the AST retains docstrings
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go file: %w", err)
	}

	// Initialize as an empty slice so JSON serializes as [] instead of null
	chunks := []Chunk{}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {

		// 1. EXTRACT FUNCTIONS AND METHODS
		case *ast.FuncDecl:
			// Capture the start position
			startPos := x.Pos()
			// If the function has a docstring above it, expand the chunk to include it
			if x.Doc != nil {
				startPos = x.Doc.Pos()
			}

			start := fset.Position(startPos).Offset
			end := fset.Position(x.End()).Offset

			chunk := Chunk{
				FilePath: filePath,
				Type:     "function",
				Content:  string(src[start:end]),
				Metadata: map[string]string{
					"name": x.Name.Name,
				},
			}

			// Check if it is a method (bound to a struct)
			if x.Recv != nil && len(x.Recv.List) > 0 {
				chunk.Type = "method"

				// Determine if the receiver is passed by pointer or by value
				switch t := x.Recv.List[0].Type.(type) {
				case *ast.StarExpr: // Pointer receiver: (s *Service)
					if ident, ok := t.X.(*ast.Ident); ok {
						chunk.Metadata["receiver"] = ident.Name
					}
				case *ast.Ident: // Value receiver: (s Service)
					chunk.Metadata["receiver"] = t.Name
				}
			}

			chunks = append(chunks, chunk)

		// 2. EXTRACT STRUCTS AND INTERFACES
		case *ast.GenDecl:
			// We only care about 'type' declarations (ignore var/const blocks)
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {

						// GenDecl holds the docstring for the entire type block
						startPos := x.Pos()
						if x.Doc != nil {
							startPos = x.Doc.Pos()
						}

						start := fset.Position(startPos).Offset
						end := fset.Position(typeSpec.End()).Offset

						chunkType := "type"
						switch typeSpec.Type.(type) {
						case *ast.StructType:
							chunkType = "struct"
						case *ast.InterfaceType:
							chunkType = "interface"
						}

						chunks = append(chunks, Chunk{
							FilePath: filePath,
							Type:     chunkType,
							Content:  string(src[start:end]),
							Metadata: map[string]string{
								"name": typeSpec.Name.Name,
							},
						})
					}
				}
			}
		}
		return true
	})

	return chunks, nil
}
