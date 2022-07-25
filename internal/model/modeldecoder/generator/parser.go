// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package generator

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

// Parsed contains information about a parsed package,
// including the package name, type information
// and some pre-defined variables
type Parsed struct {
	// package name
	pkgName string
	// parsed structs from loading types from the provided package
	structTypes map[string]structType
	// parsed pattern variables
	patternVariables map[string]string
	// parsed enumeration values
	enumVariables map[string][]string
}

type structType struct {
	name    string
	comment string
	fields  []structField
}

type structField struct {
	*types.Var
	tag     reflect.StructTag
	comment string
}

// Parse loads the Go package named by the given package pattern
// and creates a parsed struct containing the package name,
// type information and some pre-defined variables
// It returns an error if the package cannot be successfully loaded
// or parsed
func Parse(pkgPattern string) (*Parsed, error) {
	pkg, err := loadPackage(pkgPattern)
	if err != nil {
		return nil, err
	}
	parsed := Parsed{
		structTypes:      make(map[string]structType),
		patternVariables: make(map[string]string),
		enumVariables:    make(map[string][]string),
		pkgName:          pkg.Types.Name()}
	err = parse(pkg, &parsed)
	return &parsed, err
}

func loadPackage(pkg string) (*packages.Package, error) {
	cfg := packages.Config{Mode: packages.NeedTypes | packages.NeedSyntax |
		packages.NeedTypesInfo | packages.NeedImports}
	pkgs, err := packages.Load(&cfg, pkg)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, errors.New("packages load error")
	}
	return pkgs[0], nil
}

func parse(pkg *packages.Package, parsed *Parsed) error {
	for _, file := range pkg.Syntax {
		// parse type comments
		typeComments := make(map[int]string)
		for _, c := range file.Comments {
			typeComments[int(c.End())] = trimComment(c.Text())
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			switch genDecl.Tok {
			case token.VAR:
				for _, spec := range genDecl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, expr := range valueSpec.Values {
						name := valueSpec.Names[i].Name
						var err error
						switch v := expr.(type) {
						case *ast.BasicLit:
							if strings.HasPrefix(name, "pattern") {
								parsed.patternVariables[name], err = strconv.Unquote(v.Value)
							}
						case *ast.CompositeLit:
							if strings.HasPrefix(name, "enum") {
								elems := make([]string, len(v.Elts))
								for i := 0; i < len(v.Elts); i++ {
									elems[i], err = strconv.Unquote(v.Elts[i].(*ast.BasicLit).Value)
								}
								parsed.enumVariables[name] = elems
							}
						}
						if err != nil {
							return err
						}
					}
				}
			case token.TYPE:
				// find comments for the generic declaration node, in the format of
				// //MyComment
				// type MyComment struct {..}
				var genDeclComment string
				if c, ok := typeComments[int(genDecl.Pos()-1)]; ok {
					genDeclComment = c
				}
				// iterate through the type declaration for this generic declaration node
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					obj := pkg.TypesInfo.Defs[typeSpec.Name]
					if obj == nil {
						continue
					}
					var st structType
					st.name = obj.Name()
					// find comments for the specific type declaration, in the format of
					// type (
					//	 //MyComment
					//   MyComment struct {..}
					// )
					// fallback to generic declaration comment otherwise if it starts with the type name
					if c, ok := typeComments[int(genDecl.Pos()-1)]; ok && st.name == strings.Split(c, " ")[0] {
						st.comment = c
					} else if st.name == strings.Split(genDeclComment, " ")[0] {
						st.comment = genDeclComment
					}
					// find field comments (ignoring line comments)
					fieldComments := make(map[string]string)
					if st, ok := typeSpec.Type.(*ast.StructType); ok {
						for _, f := range st.Fields.List {
							if f.Doc != nil && len(f.Doc.Text()) > 0 {
								fieldComments[f.Names[0].Name] = trimComment(f.Doc.Text())
							}
						}
					}
					named := obj.(*types.TypeName).Type().(*types.Named)
					typesStruct, ok := named.Underlying().(*types.Struct)
					if !ok {
						return fmt.Errorf("unhandled type %T", named.Underlying())
					}
					numFields := typesStruct.NumFields()
					structFields := make([]structField, 0, numFields)
					for i := 0; i < numFields; i++ {
						structField := structField{
							Var: typesStruct.Field(i),
							tag: reflect.StructTag(typesStruct.Tag(i)),
						}
						if c, ok := fieldComments[structField.Name()]; ok {
							structField.comment = c
						}
						structFields = append(structFields, structField)
					}
					st.fields = structFields
					parsed.structTypes[obj.Type().String()] = st
				}
			}
		}
	}
	return nil
}

func trimComment(c string) string {
	c = strings.ReplaceAll(c, "\n", " ")
	return strings.TrimSuffix(c, " ")
}
