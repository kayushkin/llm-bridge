// genpy generates Python dataclasses from the Go types in msg/.
//
// Usage: go run ./cmd/genpy > py/llm_bridge_types/msg.py
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	msgDir := filepath.Join("msg")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, msgDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	pkg, ok := pkgs["msg"]
	if !ok {
		fmt.Fprintf(os.Stderr, "package msg not found\n")
		os.Exit(1)
	}

	// Collect files sorted by name (matches TS output order).
	var fileNames []string
	fileMap := map[string]*ast.File{}
	for name, f := range pkg.Files {
		base := filepath.Base(name)
		fileNames = append(fileNames, base)
		fileMap[base] = f
	}
	sort.Strings(fileNames)

	// Header.
	fmt.Println(`from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any`)

	for _, name := range fileNames {
		f := fileMap[name]
		emitFile(name, f, fset)
	}
}

// emitFile processes a single Go source file and emits Python declarations.
func emitFile(name string, f *ast.File, fset *token.FileSet) {
	// Collect declarations in source order.
	type decl struct {
		pos  token.Pos
		emit func()
	}
	var decls []decl

	for _, d := range f.Decls {
		switch gd := d.(type) {
		case *ast.GenDecl:
			if gd.Tok == token.TYPE {
				for _, spec := range gd.Specs {
					ts := spec.(*ast.TypeSpec)
					switch typ := ts.Type.(type) {
					case *ast.StructType:
						s := ts   // capture
						t := typ  // capture
						doc := gd.Doc
						if ts.Doc != nil {
							doc = ts.Doc
						}
						decls = append(decls, decl{s.Pos(), func() { emitDataclass(s.Name.Name, t, doc) }})
					case *ast.Ident:
						// type X string  →  type alias
						s := ts
						t := typ
						doc := gd.Doc
						if ts.Doc != nil {
							doc = ts.Doc
						}
						decls = append(decls, decl{s.Pos(), func() { emitTypeAlias(s.Name.Name, t, doc) }})
					}
				}
			}
			if gd.Tok == token.CONST {
				g := gd // capture
				decls = append(decls, decl{g.Pos(), func() { emitConsts(g) }})
			}
		}
	}

	if len(decls) == 0 {
		return
	}

	// Sort by source position (should already be, but be safe).
	sort.Slice(decls, func(i, j int) bool { return decls[i].pos < decls[j].pos })

	fmt.Printf("\n\n# %s\n# source: %s\n", strings.Repeat("─", 40), name)

	for _, d := range decls {
		d.emit()
	}
}

// emitTypeAlias emits: TypeName = str
func emitTypeAlias(name string, ident *ast.Ident, doc *ast.CommentGroup) {
	pyType := goTypeToPython(ident.Name)
	fmt.Println()
	if doc != nil {
		fmt.Printf("# %s\n", cleanComment(doc))
	}
	fmt.Printf("%s = %s\n", name, pyType)
}

// emitConsts emits Python constants from a Go const block.
func emitConsts(gd *ast.GenDecl) {
	// Track the iota type for the block.
	var constType string
	first := true

	for _, spec := range gd.Specs {
		vs := spec.(*ast.ValueSpec)

		// Determine the type name (explicit or inherited from iota group).
		if vs.Type != nil {
			constType = exprString(vs.Type)
		}

		pyType := ""
		if constType != "" {
			pyType = constType
		}

		for i, n := range vs.Names {
			if !ast.IsExported(n.Name) {
				continue
			}
			if first {
				fmt.Println()
				// Emit doc comment for the const block if present.
				if gd.Doc != nil {
					fmt.Printf("# %s\n", cleanComment(gd.Doc))
				}
				first = false
			}

			val := ""
			if i < len(vs.Values) {
				val = constValueString(vs.Values[i])
			}
			if val == "" {
				continue
			}
			if pyType != "" {
				fmt.Printf("%s: %s = %s\n", n.Name, pyType, val)
			} else {
				fmt.Printf("%s = %s\n", n.Name, val)
			}
		}
	}
}

// emitDataclass emits a Python dataclass from a Go struct.
func emitDataclass(name string, st *ast.StructType, doc *ast.CommentGroup) {
	fmt.Println()
	fmt.Println()
	if doc != nil {
		// Emit as docstring inside class.
	}
	fmt.Printf("@dataclass(kw_only=True)\nclass %s:\n", name)
	if doc != nil {
		fmt.Printf("    \"\"\"%s\"\"\"\n", cleanComment(doc))
	}

	if st.Fields == nil || len(st.Fields.List) == 0 {
		fmt.Println("    pass")
		return
	}

	for _, f := range st.Fields.List {
		jsonName, omitempty := parseJSONTag(f.Tag)
		if jsonName == "" || jsonName == "-" {
			// Struct fields without JSON tags (like ValidationFailure).
			if len(f.Names) > 0 {
				fieldName := toSnakeCase(f.Names[0].Name)
				pyType := goFieldTypeToPython(f.Type, false)
				fmt.Printf("    %s: %s\n", fieldName, pyType)
			}
			continue
		}

		isPointer := isPointerType(f.Type)
		optional := omitempty || isPointer
		pyType := goFieldTypeToPython(f.Type, false)

		comment := ""
		if f.Comment != nil {
			comment = "  " + cleanInlineComment(f.Comment)
		}

		if optional {
			fmt.Printf("    %s: %s | None = None%s\n", jsonName, pyType, comment)
		} else {
			fmt.Printf("    %s: %s%s\n", jsonName, pyType, comment)
		}
	}
}

// --- Type mapping ---

func goTypeToPython(goType string) string {
	switch goType {
	case "string":
		return "str"
	case "int", "int64":
		return "int"
	case "float64":
		return "float"
	case "bool":
		return "bool"
	default:
		return goType
	}
}

func goFieldTypeToPython(expr ast.Expr, inSlice bool) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return goTypeToPython(t.Name)
	case *ast.StarExpr:
		// Pointer type — unwrap. The caller handles optionality.
		return goFieldTypeToPython(t.X, inSlice)
	case *ast.ArrayType:
		elemType := goFieldTypeToPython(t.Elt, true)
		return "list[" + elemType + "]"
	case *ast.MapType:
		keyType := goFieldTypeToPython(t.Key, false)
		valType := goFieldTypeToPython(t.Value, false)
		return "dict[" + keyType + ", " + valType + "]"
	case *ast.SelectorExpr:
		pkg := exprString(t.X)
		sel := t.Sel.Name
		switch pkg + "." + sel {
		case "json.RawMessage":
			return "dict[str, Any]"
		case "time.Time":
			return "str"
		}
		return sel
	default:
		return "Any"
	}
}

func isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// --- JSON tag parsing ---

func parseJSONTag(tag *ast.BasicLit) (name string, omitempty bool) {
	if tag == nil {
		return "", false
	}
	raw := tag.Value
	// Strip backticks.
	raw = strings.Trim(raw, "`")

	// Find json:"..." in the tag.
	idx := strings.Index(raw, `json:"`)
	if idx < 0 {
		return "", false
	}
	rest := raw[idx+6:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return "", false
	}
	jsonVal := rest[:end]

	parts := strings.Split(jsonVal, ",")
	name = parts[0]
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty
}

// --- Helpers ---

func exprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	default:
		return ""
	}
}

func constValueString(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		// Likely iota or another const reference.
		return ""
	default:
		return ""
	}
}

func cleanComment(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	var lines []string
	for _, c := range cg.List {
		text := c.Text
		text = strings.TrimPrefix(text, "// ")
		text = strings.TrimPrefix(text, "//")
		lines = append(lines, text)
	}
	return strings.Join(lines, " ")
}

func cleanInlineComment(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	var parts []string
	for _, c := range cg.List {
		text := strings.TrimPrefix(c.Text, "// ")
		text = strings.TrimPrefix(text, "//")
		parts = append(parts, text)
	}
	return "# " + strings.Join(parts, " ")
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
