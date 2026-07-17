package cqrslint

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel load errors. Defined as package-level values so callers can match
// them with errors.Is (satisfies err113: no dynamic errors).
var (
	// ErrNotADirectory is returned when the load target is not a directory.
	ErrNotADirectory = errors.New("path is not a directory")
	// ErrNoGoSources is returned when a directory contains no non-test .go files.
	ErrNoGoSources = errors.New("no non-test .go sources found")
)

// Package is a parsed Go package: its name, the absolute root it was loaded
// from (used to render relative file paths in findings), the file set (for
// positions), and the non-test source files.
type Package struct {
	Name  string
	Root  string
	Fset  *token.FileSet
	Files []*ast.File
}

// LoadPackage parses every non-test .go file in dir (recursively) using
// go/parser. It performs no type checking and imports nothing beyond the
// standard library, so it is hermetic and fast.
func LoadPackage(dir string) (*Package, error) {
	root, resolveErr := filepath.Abs(dir)
	if resolveErr != nil {
		return nil, fmt.Errorf("resolve %q: %w", dir, resolveErr)
	}

	info, statErr := os.Stat(root)
	if statErr != nil {
		return nil, fmt.Errorf("stat %q: %w", root, statErr)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %q", ErrNotADirectory, root)
	}

	fset := token.NewFileSet()

	files, pkgName, walkErr := parseGoSources(fset, root)
	if walkErr != nil {
		return nil, fmt.Errorf("load %q: %w", root, walkErr)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%w under %q", ErrNoGoSources, root)
	}

	return &Package{
		Name:  pkgName,
		Root:  root,
		Fset:  fset,
		Files: files,
	}, nil
}

// parseGoSources walks root recursively and parses every non-test .go file,
// returning the parsed files and the discovered package name (from the first
// file; all files in a Go package share the same name).
func parseGoSources(fset *token.FileSet, root string) ([]*ast.File, string, error) {
	var files []*ast.File

	var pkgName string

	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return skipSubdirectory(entry.Name(), filepath.Base(root))
		}

		if !isGoSource(path) {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return fmt.Errorf("parse %q: %w", path, parseErr)
		}

		if pkgName == "" {
			pkgName = file.Name.Name
		}

		files = append(files, file)

		return nil
	})

	return files, pkgName, walkErr
}

// skipSubdirectory reports whether a directory should be pruned from the walk.
// Vendored and hidden directories are never part of a CQRS package's source set.
// The root itself is never skipped (even if it is named "vendor").
func skipSubdirectory(dirName, rootBase string) error {
	if dirName == rootBase {
		return nil
	}

	if dirName == "vendor" || strings.HasPrefix(dirName, ".") {
		return filepath.SkipDir
	}

	return nil
}

// isGoSource reports whether path is a production (non-test) .go file.
func isGoSource(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}

	return !strings.HasSuffix(path, "_test.go")
}

// RelFile returns a path relative to the package root, falling back to the
// absolute path if it cannot be made relative. Used for stable findings output
// regardless of where the tool is invoked from.
func (p *Package) RelFile(absPath string) string {
	rel, err := filepath.Rel(p.Root, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}

	return rel
}

// PositionFor resolves an ast.Node to a (file, line) pair relative to the root.
func (p *Package) PositionFor(node ast.Node) (string, int) {
	position := p.Fset.Position(node.Pos())

	return p.RelFile(position.Filename), position.Line
}
