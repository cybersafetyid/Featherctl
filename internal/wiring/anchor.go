package wiring

import (
	"errors"
	"fmt"
	"go/token"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

// anchorKind describes the kind of list an injection point resolves to.
type anchorKind int

const (
	// kindStatements is a list of statements inside a block.
	kindStatements anchorKind = iota
	// kindFields is a list of struct fields.
	kindFields
	// kindDecls is the list of top level declarations of a file.
	kindDecls
)

// anchor is the resolved insertion point for a marker comment.
type anchor struct {
	kind   anchorKind
	file   *dst.File
	block  *dst.BlockStmt
	fields *dst.FieldList
	index  int
}

// findAnchor walks the decorated tree looking for the node that owns the
// marker comment and resolves it to an insertion point.
func findAnchor(file *dst.File, inj Injection) (*anchor, error) {
	marker := strings.TrimSpace(inj.Marker)
	if marker == "" {
		return nil, ErrEmptyMarker
	}

	var (
		found       *anchor
		unsupported string
	)

	dstutil.Apply(file, func(c *dstutil.Cursor) bool {
		if found != nil || unsupported != "" {
			return false
		}

		node := c.Node()
		if node == nil {
			return true
		}

		// A comment that sits at the very head or tail of a brace-enclosed
		// list is attached to the list itself rather than to one of its
		// entries. Both are valid injection points.
		switch n := node.(type) {
		case *dst.BlockStmt:
			if isMarker(n.Decs.Start, marker) || isMarker(n.Decs.Lbrace, marker) {
				found = &anchor{kind: kindStatements, block: n, index: 0}
				return false
			}
			if isMarker(n.Decs.End, marker) {
				found = &anchor{kind: kindStatements, block: n, index: len(n.List)}
				return false
			}
		case *dst.FieldList:
			if isMarker(n.Decs.Opening, marker) {
				found = &anchor{kind: kindFields, fields: n, index: 0}
				return false
			}
			if isMarker(n.Decs.End, marker) {
				found = &anchor{kind: kindFields, fields: n, index: len(n.List)}
				return false
			}
		case *dst.File:
			if isMarker(n.Decs.Start, marker) {
				found = &anchor{kind: kindDecls, file: n, index: 0}
				return false
			}
			if isMarker(n.Decs.End, marker) {
				found = &anchor{kind: kindDecls, file: n, index: len(n.Decls)}
				return false
			}
		}

		decs := node.Decorations()
		if decs == nil {
			return true
		}

		var trailing bool
		switch {
		case isMarker(decs.Start, marker):
		case isMarker(decs.End, marker):
			trailing = true
		default:
			return true
		}

		index := c.Index()
		if index < 0 {
			unsupported = fmt.Sprintf("attached to a %T that is not part of a list", node)
			return false
		}
		if trailing {
			index++
		}

		switch parent := c.Parent().(type) {
		case *dst.BlockStmt:
			found = &anchor{kind: kindStatements, block: parent, index: index}
		case *dst.FieldList:
			found = &anchor{kind: kindFields, fields: parent, index: index}
		case *dst.File:
			found = &anchor{kind: kindDecls, file: parent, index: index}
		default:
			unsupported = fmt.Sprintf("attached to a %T whose parent is a %T", node, c.Parent())
		}
		return false
	}, nil)

	switch {
	case found != nil:
		return found, nil
	case unsupported != "":
		return nil, &MarkerPositionError{Path: inj.Path, Marker: marker, Position: unsupported}
	default:
		return nil, &MarkerNotFoundError{Path: inj.Path, Marker: marker}
	}
}

// insert parses code and splices it into the resolved insertion point.
func (a *anchor) insert(code string) error {
	switch a.kind {
	case kindFields:
		fields, err := parseFields(code)
		if err != nil {
			return err
		}
		a.fields.List = insertAt(a.fields.List, a.index, fields...)
	case kindStatements:
		stmts, err := parseStatements(code)
		if err != nil {
			return err
		}
		a.block.List = insertAt(a.block.List, a.index, stmts...)
	case kindDecls:
		decls, err := parseDecls(code)
		if err != nil {
			return err
		}
		a.file.Decls = insertAt(a.file.Decls, a.index, decls...)
	default:
		return errors.New("wiring: internal error: unresolved insertion point")
	}
	return nil
}

// parseStatements parses code as the body of a function.
func parseStatements(code string) ([]dst.Stmt, error) {
	src := "package injected\n\nfunc injected() {\n" + trimTrailingNewlines(code) + "\n}\n"
	file, err := decorator.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("wiring: invalid statement code: %w", err)
	}

	decl, ok := file.Decls[0].(*dst.FuncDecl)
	if !ok || decl.Body == nil {
		return nil, errors.New("wiring: invalid statement code")
	}

	stmts := decl.Body.List
	for _, stmt := range stmts {
		// NewLine keeps the statement on its own line even when the anchor is
		// the opening brace of a block.
		stmt.Decorations().Before = dst.NewLine
		stmt.Decorations().After = dst.NewLine
	}
	return stmts, nil
}

// parseFields parses code as the field list of a struct.
func parseFields(code string) ([]*dst.Field, error) {
	src := "package injected\n\ntype injected struct {\n" + trimTrailingNewlines(code) + "\n}\n"
	file, err := decorator.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("wiring: invalid struct field code: %w", err)
	}

	decl, ok := file.Decls[0].(*dst.GenDecl)
	if !ok || len(decl.Specs) != 1 {
		return nil, errors.New("wiring: invalid struct field code")
	}
	spec, ok := decl.Specs[0].(*dst.TypeSpec)
	if !ok {
		return nil, errors.New("wiring: invalid struct field code")
	}
	structure, ok := spec.Type.(*dst.StructType)
	if !ok {
		return nil, errors.New("wiring: invalid struct field code")
	}

	fields := structure.Fields.List
	for i, field := range fields {
		// NewLine keeps the field — and its doc comment — off the opening
		// brace of the struct when it is inserted as the first field.
		field.Decs.Before = dst.NewLine
		if i == len(fields)-1 {
			field.Decs.After = dst.EmptyLine
			continue
		}
		field.Decs.After = dst.NewLine
	}
	return fields, nil
}

// parseDecls parses code as top level declarations.
func parseDecls(code string) ([]dst.Decl, error) {
	src := "package injected\n\n" + trimTrailingNewlines(code) + "\n"
	file, err := decorator.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("wiring: invalid declaration code: %w", err)
	}
	if len(file.Decls) == 0 {
		return nil, errors.New("wiring: invalid declaration code")
	}
	return file.Decls, nil
}

// addImport appends path to the file's import block unless it is already
// imported. It reports whether the file was changed.
func addImport(file *dst.File, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*dst.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			imp, ok := spec.(*dst.ImportSpec)
			if !ok {
				continue
			}
			if unquoteImport(imp.Path.Value) == path {
				return false
			}
		}

		imp := &dst.ImportSpec{Path: &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
		imp.Decs.Before = dst.NewLine
		imp.Decs.After = dst.NewLine
		gen.Specs = append(gen.Specs, imp)
		// A single spec import must grow an import block before it can hold a
		// second path.
		gen.Lparen = true
		gen.Rparen = true
		return true
	}

	imp := &dst.ImportSpec{Path: &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
	imp.Decs.Before = dst.NewLine
	imp.Decs.After = dst.NewLine

	gen := &dst.GenDecl{Tok: token.IMPORT, Lparen: true, Rparen: true, Specs: []dst.Spec{imp}}
	gen.Decs.Before = dst.EmptyLine
	gen.Decs.After = dst.NewLine

	file.Decls = append([]dst.Decl{gen}, file.Decls...)
	return true
}

// isMarker reports whether any decoration is exactly the marker comment.
func isMarker(decs dst.Decorations, marker string) bool {
	for _, dec := range decs {
		if strings.TrimSpace(dec) == marker {
			return true
		}
	}
	return false
}

// unquoteImport turns an import literal into its path.
func unquoteImport(value string) string {
	path, err := strconv.Unquote(value)
	if err != nil {
		return strings.Trim(value, "\"`")
	}
	return path
}

// insertAt returns list with values spliced in at index, clamped to the list
// bounds.
func insertAt[T any](list []T, index int, values ...T) []T {
	if index < 0 {
		index = 0
	}
	if index > len(list) {
		index = len(list)
	}

	out := make([]T, 0, len(list)+len(values))
	out = append(out, list[:index]...)
	out = append(out, values...)
	out = append(out, list[index:]...)
	return out
}

// trimTrailingNewlines removes trailing newlines so generated code can be
// wrapped in a synthetic file without growing extra blank lines.
func trimTrailingNewlines(code string) string {
	return strings.TrimRight(code, "\n")
}
