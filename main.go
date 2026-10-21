package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
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
	imports := importPaths(node)

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

			if types := paramTypes(x.Type.Params, imports); len(types) > 0 {
				chunk.Metadata["param_types"] = strings.Join(types, ",")
			}
			if h := handles(x, imports); h != "" {
				chunk.Metadata["handles"] = h
			}
			chunk.Metadata["package"] = node.Name.Name

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
								"name":    typeSpec.Name.Name,
								"package": node.Name.Name,
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

// importPaths maps the name each import is used under to its path.
func importPaths(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if v := name; len(v) > 1 && v[0] == 'v' && strings.Trim(v[1:], "0123456789") == "" {
			name = path.Base(path.Dir(p)) // github.com/x/y/v2 is used as y
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = p
	}
	return out
}

var builtinTypes = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true, "error": true, "float32": true, "float64": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true, "rune": true, "string": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true, "any": true,
}

// typeName is the named type an expression stands for, looking through
// pointers, slices, arrays, channels, variadics and generic instances: Foo for
// *Foo, and path/to/pkg.Foo for pkg.Foo. Builtins, maps and function types give "".
func typeName(e ast.Expr, imports map[string]string) string {
	switch t := e.(type) {
	case *ast.Ident:
		if builtinTypes[t.Name] {
			return ""
		}
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			if p, ok := imports[x.Name]; ok {
				return p + "." + t.Sel.Name
			}
			return x.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return typeName(t.X, imports)
	case *ast.ArrayType:
		return typeName(t.Elt, imports)
	case *ast.Ellipsis:
		return typeName(t.Elt, imports)
	case *ast.ChanType:
		return typeName(t.Value, imports)
	case *ast.IndexExpr:
		return typeName(t.X, imports)
	case *ast.IndexListExpr:
		return typeName(t.X, imports)
	}
	return ""
}

// paramTypes lists, in order and without repeats, the named types of the parameters.
func paramTypes(fields *ast.FieldList, imports map[string]string) []string {
	if fields == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, f := range fields.List {
		if n := typeName(f.Type, imports); n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// handles is the type a function or method handles when it is a handler or
// listener entry (Handle, Execute, Handle*, On*): its first parameter after a
// context.Context. A function with no such parameter handles nothing.
func handles(fn *ast.FuncDecl, imports map[string]string) string {
	name := fn.Name.Name
	ok := name == "Handle" || name == "Execute" || name == "handle" || name == "execute"
	for _, prefix := range []string{"Handle", "On", "handle", "on"} {
		if len(name) > len(prefix) && strings.HasPrefix(name, prefix) && name[len(prefix)] >= 'A' && name[len(prefix)] <= 'Z' {
			ok = true
		}
	}
	if !ok || fn.Type.Params == nil {
		return ""
	}
	for _, f := range fn.Type.Params.List {
		n := typeName(f.Type, imports)
		if n == "context.Context" || n == "" && isContext(f.Type) {
			continue
		}
		return n
	}
	return ""
}

func isContext(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Context"
}
