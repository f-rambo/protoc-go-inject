package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Annotation struct {
	Type    string // goimport, gofield, or gotags
	Content string
}

func parseAnnotations(line string) []Annotation {
	var annotations []Annotation

	// Regular expressions for different annotation types
	goimportRe := regexp.MustCompile(`@goimport:\s*"([^"]+)"`)
	gofieldRe := regexp.MustCompile(`@gofield:\s*(.+)`)
	gotagsRe := regexp.MustCompile(`@gotags:\s*(.+)`)
	gotypeRe := regexp.MustCompile(`type\s+(\w+)\s+struct`)

	if match := goimportRe.FindStringSubmatch(line); len(match) > 1 {
		annotations = append(annotations, Annotation{Type: "goimport", Content: match[1]})
	}
	if match := gofieldRe.FindStringSubmatch(line); len(match) > 1 {
		annotations = append(annotations, Annotation{Type: "gofield", Content: match[1]})
	}
	if match := gotagsRe.FindStringSubmatch(line); len(match) > 1 {
		annotations = append(annotations, Annotation{Type: "gotags", Content: match[1]})
	}
	if match := gotypeRe.FindStringSubmatch(line); len(match) > 1 {
		annotations = append(annotations, Annotation{Type: "gotype", Content: match[1]})
	}

	return annotations
}

func createFieldFromString(fieldStr string) *ast.Field {
	parts := strings.Fields(fieldStr)
	if len(parts) == 1 { // Embedded type
		return &ast.Field{
			Type: ast.NewIdent(parts[0]),
		}
	} else if len(parts) >= 2 { // Named field with type
		return &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(parts[0])},
			Type:  ast.NewIdent(parts[1]),
		}
	}
	return nil
}

// parseTags parses a Go struct tag string into a map of key-value pairs
func parseTags(tagStr string) map[string]string {
	tags := make(map[string]string)
	tagStr = strings.Trim(tagStr, "`")

	// Use regex to find key-value pairs like protobuf:"..." or json:"..."
	re := regexp.MustCompile(`(\w+):"([^"]+)"`)

	// Find all matches
	matches := re.FindAllStringSubmatch(tagStr, -1)

	for _, match := range matches {
		if len(match) == 3 { // Ensure there are key and value
			key := match[1]
			val := match[2]
			tags[key] = val
		}
	}
	return tags
}

// formatTags converts a map of tags back to a tag string
func formatTags(tags map[string]string) string {
	var parts []string
	for key, value := range tags {
		parts = append(parts, fmt.Sprintf(`%s:"%s"`, key, value))
	}
	return strings.Join(parts, " ")
}

// getEmbeddedStructName gets the name of an embedded struct
func getEmbeddedStructName(field *ast.Field) string {
	// If field has names, it's not an embedded struct
	if len(field.Names) > 0 {
		return ""
	}

	// Handle different types of embedded structs
	switch t := field.Type.(type) {
	case *ast.Ident:
		// Simple embedded struct (e.g., Model)
		return t.Name
	case *ast.SelectorExpr:
		// Qualified embedded struct (e.g., gorm.Model)
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	}
	return ""
}

func processFile(inputPath string) error {
	// Read the input file
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Parse the Go file
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, inputPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %v", err)
	}

	// Create maps to store unique imports and fields
	imports := make(map[string]bool)
	fields := make(map[string]map[string]string)
	tags := make(map[string]map[string]string)

	// Process annotations
	goTypeStr := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		annotations := parseAnnotations(line)
		if len(annotations) == 0 {
			continue
		}
		for _, ann := range annotations {
			switch ann.Type {
			case "goimport":
				imports[ann.Content] = true
			case "gotype":
				goTypeStr = ann.Content
				fields[goTypeStr] = make(map[string]string)
				tags[goTypeStr] = make(map[string]string)
			case "gofield":
				fields[goTypeStr][ann.Content] = ann.Content
			case "gotags":
				// Extract field name from the line by looking for protobuf field names
				fieldMatch := regexp.MustCompile(`name=(\w+)`).FindStringSubmatch(line)
				if len(fieldMatch) > 1 {
					fieldName := fieldMatch[1]
					tags[goTypeStr][strings.ToLower(strings.ReplaceAll(fieldName, "_", ""))] = strings.TrimSpace(ann.Content)
				}
			}
		}
	}

	// Add new imports
	for imp := range imports {
		importSpec := &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("%q", imp),
			},
		}

		// Find or create import declaration
		var importDecl *ast.GenDecl
		for _, decl := range astFile.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
				importDecl = genDecl
				break
			}
		}

		if importDecl == nil {
			importDecl = &ast.GenDecl{
				Tok:    token.IMPORT,
				Lparen: 1, // Multi-line import block
			}
			astFile.Decls = append([]ast.Decl{importDecl}, astFile.Decls...)
		}

		// Check for duplicate imports
		isDuplicate := false
		for _, spec := range importDecl.Specs {
			if impSpec, ok := spec.(*ast.ImportSpec); ok {
				if impSpec.Path.Value == fmt.Sprintf("%q", imp) {
					isDuplicate = true
					break
				}
			}
		}

		if !isDuplicate {
			importDecl.Specs = append(importDecl.Specs, importSpec)
		}
	}

	// Process type declarations and add fields/tags
	for _, decl := range astFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						// Get the struct name
						structName := typeSpec.Name.Name

						// Add new fields
						existingFields := make(map[string]bool)
						for _, field := range structType.Fields.List {
							if len(field.Names) > 0 {
								existingFields[field.Names[0].Name] = true
							} else {
								// Handle embedded struct
								if embeddedName := getEmbeddedStructName(field); embeddedName != "" {
									existingFields[embeddedName] = true
								}
							}
						}
						for _, fieldStr := range fields[structName] {
							field := createFieldFromString(fieldStr)
							if field != nil {
								fieldName := ""
								if len(field.Names) > 0 {
									fieldName = field.Names[0].Name
								} else {
									fieldName = getEmbeddedStructName(field)
								}

								// Check for duplicates
								isDuplicate := false
								if fieldName != "" && existingFields[fieldName] {
									isDuplicate = true
								}

								if !isDuplicate {
									structType.Fields.List = append(structType.Fields.List, field)
									if fieldName != "" {
										existingFields[fieldName] = true
									}
								}
							}
						}

						// Update tags
						for _, field := range structType.Fields.List {
							if len(field.Names) > 0 {
								if newTagStr, exists := tags[structName][strings.ToLower(field.Names[0].Name)]; exists {
									// Parse existing and new tags
									existingTags := make(map[string]string)
									if field.Tag != nil {
										existingTags = parseTags(field.Tag.Value)
									}

									newTags := parseTags(newTagStr)

									// Merge tags, new tags take precedence
									for k, v := range newTags {
										existingTags[k] = v
									}

									// Set the combined tags
									field.Tag = &ast.BasicLit{
										Kind:  token.STRING,
										Value: fmt.Sprintf("`%s`", formatTags(existingTags)),
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Write the modified AST to output file
	outFile, err := os.Create(inputPath + ".enhanced")
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	if err := format.Node(outFile, fset, astFile); err != nil {
		return fmt.Errorf("failed to write output: %v", err)
	}

	return nil
}

func printHelp() {
	fmt.Println("protoc-go-inject - A tool to inject custom annotations into protobuf-generated Go files")
	fmt.Println("\nUsage:")
	fmt.Println("  protoc-go-inject [options] <pb.go files...>")
	fmt.Println("\nOptions:")
	fmt.Println("  -h, --help     Show this help message")
	fmt.Println("\nExample:")
	fmt.Println("  protoc-go-inject a.pb.go b.pb.go")
	fmt.Println("\nSupported Annotations:")
	fmt.Println("  @goimport: Add new package imports")
	fmt.Println("    Example: // @goimport: \"gorm.io/gorm\"")
	fmt.Println("\n  @gofield: Add new struct fields")
	fmt.Println("    Example: // @gofield: gorm.Model")
	fmt.Println("    Example: // @gofield: LastName string")
	fmt.Println("\n  @gotags: Append or modify struct field tags")
	fmt.Println("    Example: // @gotags: gorm:\"column:id;primaryKey;AUTO_INCREMENT\"")
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		printHelp()
		if len(os.Args) < 2 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Process each input file
	for _, fpath := range os.Args[1:] {
		fmt.Printf("Processing %s...\n", fpath)

		// Get absolute path
		absPath, err := filepath.Abs(fpath)
		if err != nil {
			fmt.Printf("Error getting absolute path for %s: %v\n", fpath, err)
			continue
		}

		if err := processFile(absPath); err != nil {
			fmt.Printf("Error processing %s: %v\n", fpath, err)
			continue
		}

		// Read the enhanced file
		enhancedContent, err := os.ReadFile(absPath + ".enhanced")
		if err != nil {
			fmt.Printf("Error reading enhanced file for %s: %v\n", fpath, err)
			continue
		}

		// Write back to original file
		if err := os.WriteFile(absPath, enhancedContent, 0644); err != nil {
			fmt.Printf("Error writing back to %s: %v\n", fpath, err)
			continue
		}

		// Remove the .enhanced file
		if err := os.Remove(absPath + ".enhanced"); err != nil {
			fmt.Printf("Warning: Could not remove enhanced file for %s: %v\n", fpath, err)
		}

		fmt.Printf("Successfully processed %s\n", fpath)
	}
}
