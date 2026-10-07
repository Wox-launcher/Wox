// Command assetpack creates a Go overlay containing losslessly compressed assets.
// It never rewrites source resources or signed helper executables.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type listedPackage struct {
	Dir        string
	ImportPath string
	GoFiles    []string
	CgoFiles   []string
	EmbedFiles []string
	Module     *struct{ Main bool }
}

// main produces a build overlay and fails the release step if any asset is invalid.
func main() {
	output := flag.String("out", ".build/assets", "output directory for packed assets and overlay.json")
	tags := flag.String("tags", "", "build tags used by the target build")
	targetOS := flag.String("goos", runtime.GOOS, "target operating system")
	targetArch := flag.String("goarch", runtime.GOARCH, "target architecture")
	flag.Parse()
	if err := pack(*output, *tags, *targetOS, *targetArch); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// pack follows go list's platform-specific embed selection, including its hidden
// file exclusions. A new resource under any existing embed pattern needs no manifest update.
func pack(output, tags, targetOS, targetArch string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "list", "-deps", "-json", "-tags", tags, ".")
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOOS="+targetOS, "GOARCH="+targetArch, "CGO_ENABLED=1")
	listing, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list embedded assets: %w", err)
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	replacements := map[string]string{}
	var originalSize, packedSize int
	decoder := json.NewDecoder(bytes.NewReader(listing))
	for {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if pkg.Module == nil || !pkg.Module.Main || len(pkg.EmbedFiles) == 0 {
			continue
		}
		if err := validateConsumers(pkg); err != nil {
			return err
		}
		for _, name := range pkg.EmbedFiles {
			original := filepath.Join(pkg.Dir, filepath.FromSlash(name))
			if _, exists := replacements[original]; exists {
				continue
			}
			data, err := os.ReadFile(original)
			if err != nil {
				return err
			}
			compressed, err := compress(data)
			if err != nil {
				return fmt.Errorf("pack %s: %w", original, err)
			}
			digest := sha256.Sum256(compressed)
			target := filepath.Join(output, fmt.Sprintf("%x.gz", digest))
			if err := os.WriteFile(target, compressed, 0644); err != nil {
				return err
			}
			replacements[original] = target
			originalSize += len(data)
			packedSize += len(compressed)
		}
	}
	mode := filepath.Join(output, "mode.go")
	if err := os.WriteFile(mode, []byte("package assetfs\nconst packed = true\n"), 0644); err != nil {
		return err
	}
	replacements[filepath.Join(root, "internal", "assetfs", "mode.go")] = mode
	overlay, err := json.MarshalIndent(struct{ Replace map[string]string }{replacements}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "overlay.json"), overlay, 0644); err != nil {
		return err
	}
	fmt.Printf("Packed %d Wox assets: %d -> %d bytes\n", len(replacements)-1, originalSize, packedSize)
	return nil
}

// compress verifies exact reconstruction, including Authenticode signatures. The
// gzip header omits paths and timestamps so identical inputs generate identical output.
func compress(data []byte) ([]byte, error) {
	if uint64(len(data)) >= 1<<32 {
		return nil, fmt.Errorf("asset exceeds gzip metadata size limit")
	}
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(data); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		return nil, err
	}
	restored, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, restored) {
		return nil, fmt.Errorf("compressed asset failed round-trip verification")
	}
	return buffer.Bytes(), nil
}

// validateConsumers prevents a new raw go:embed consumer from accidentally seeing
// gzip bytes in release builds. Every owned embed must use the same transparent FS.
func validateConsumers(pkg listedPackage) error {
	embedded := map[string]bool{}
	wrapped := map[string]bool{}
	for _, name := range append(pkg.GoFiles, pkg.CgoFiles...) {
		tree, err := parser.ParseFile(token.NewFileSet(), filepath.Join(pkg.Dir, name), nil, parser.ParseComments)
		if err != nil {
			return err
		}
		alias := ""
		for _, imp := range tree.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == "wox/internal/assetfs" {
				alias = "assetfs"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
			}
		}
		for _, decl := range tree.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				for _, doc := range []*ast.CommentGroup{gen.Doc, value.Doc} {
					if doc == nil {
						continue
					}
					for _, comment := range doc.List {
						if strings.HasPrefix(comment.Text, "//go:embed ") || strings.HasPrefix(comment.Text, "//go:embed\t") {
							for _, n := range value.Names {
								embedded[n.Name] = true
							}
						}
					}
				}
			}
		}
		ast.Inspect(tree, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "New" {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok || alias == "" || qualifier.Name != alias {
				return true
			}
			argument, ok := call.Args[0].(*ast.Ident)
			if ok {
				wrapped[argument.Name] = true
			}
			return true
		})
	}
	for name := range embedded {
		if !wrapped[name] {
			return fmt.Errorf("%s: embedded variable %s must be wrapped with assetfs.New", pkg.ImportPath, name)
		}
	}
	return nil
}
