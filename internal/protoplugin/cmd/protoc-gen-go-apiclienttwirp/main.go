// Copyright 2020-2021 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"strings"

	"github.com/bufbuild/buf/internal/pkg/app/appproto"
	"github.com/bufbuild/buf/internal/pkg/protogenutil"
	"google.golang.org/protobuf/compiler/protogen"
)

const (
	contextPackage     = protogen.GoImportPath("context")
	httpclientPackage  = protogen.GoImportPath("github.com/bufbuild/buf/internal/pkg/transport/http/httpclient")
	twirpclientPackage = protogen.GoImportPath("github.com/bufbuild/buf/internal/pkg/transport/twirp/twirpclient")
	zapPackage         = protogen.GoImportPath("go.uber.org/zap")
	pluginName         = "apiclienttwirp"
)

func main() {
	appproto.Main(context.Background(), protogenutil.NewNamedPerGoPackageHandler(handle))
}

func handle(helper protogenutil.NamedHelper, plugin *protogen.Plugin, goPackageFileSet *protogenutil.GoPackageFileSet) error {
	if len(goPackageFileSet.Services()) == 0 {
		return nil
	}
	if err := generatePackageFile(helper, plugin, goPackageFileSet); err != nil {
		return err
	}
	for _, file := range goPackageFileSet.Files {
		if len(file.Services) > 0 {
			if err := generateServiceFile(helper, plugin, file); err != nil {
				return err
			}
		}
	}
	return nil
}

func generatePackageFile(helper protogenutil.NamedHelper, plugin *protogen.Plugin, goPackageFileSet *protogenutil.GoPackageFileSet) error {
	g, err := helper.NewPackageGeneratedFile(plugin, goPackageFileSet, pluginName)
	if err != nil {
		return err
	}
	httpClientGoIdentString := g.QualifiedGoIdent(httpclientPackage.Ident("Client"))
	newClientOptionsGoIdentString := g.QualifiedGoIdent(twirpclientPackage.Ident("NewClientOptions"))
	loggerGoIdentString := g.QualifiedGoIdent(zapPackage.Ident("Logger"))
	apiclientGoImportPath, err := helper.NewPackageGoImportPath(
		goPackageFileSet,
		"apiclient",
	)
	if err != nil {
		return err
	}
	providerGoIdent := apiclientGoImportPath.Ident("Provider")
	providerGoIdentString := g.QualifiedGoIdent(providerGoIdent)

	g.P(`// NewProvider returns a new Provider.`)
	g.P(`func NewProvider(`)
	g.P(`logger *`, loggerGoIdentString, `,`)
	g.P(`httpClient `, httpClientGoIdentString, `,`)
	g.P(`) `, providerGoIdentString, `{`)
	g.P(`return &provider{`)
	g.P(`logger: logger,`)
	g.P(`httpClient: httpClient,`)
	g.P(`}`)
	g.P(`}`)
	g.P()
	g.P(`type provider struct {`)
	g.P(`logger *`, loggerGoIdentString)
	g.P(`httpClient `, httpClientGoIdentString)
	g.P(`}`)
	g.P()

	contextGoIdentString := g.QualifiedGoIdent(contextPackage.Ident("Context"))
	apiGoImportPath, err := helper.NewPackageGoImportPath(
		goPackageFileSet,
		"api",
	)
	if err != nil {
		return err
	}

	for _, service := range goPackageFileSet.Services() {
		interfaceName := service.GoName
		interfaceGoIdent := apiGoImportPath.Ident(interfaceName)
		interfaceGoIdentString := g.QualifiedGoIdent(interfaceGoIdent)
		structName := protogenutil.GetUnexportGoName(interfaceName)
		newProtobufClientGoIdent := goPackageFileSet.GoImportPath.Ident(`New` + interfaceName + `ProtobufClient`)
		newProtobufClientGoIdentString := g.QualifiedGoIdent(newProtobufClientGoIdent)

		g.P(`func (p *provider) New`, interfaceName, `(ctx `, contextGoIdentString, `, address string) (`, interfaceGoIdentString, `, error) {`)
		g.P(`return &`, structName, `{`)
		g.P(`logger: p.logger,`)
		g.P(`client: `, newProtobufClientGoIdentString, `(`)
		g.P(`p.httpClient.ParseAddress(address),`)
		g.P(`p.httpClient,`)
		g.P(newClientOptionsGoIdentString, `()...,`)
		g.P(`),`)
		g.P(`}, nil`)
		g.P(`}`)
		g.P()
	}
	return nil
}

func generateServiceFile(helper protogenutil.NamedHelper, plugin *protogen.Plugin, file *protogen.File) error {
	g, err := helper.NewGeneratedFile(plugin, file, pluginName)
	if err != nil {
		return err
	}
	contextGoIdentString := g.QualifiedGoIdent(contextPackage.Ident("Context"))
	loggerGoIdentString := g.QualifiedGoIdent(zapPackage.Ident("Logger"))

	for _, service := range file.Services {
		interfaceName := service.GoName
		structName := protogenutil.GetUnexportGoName(interfaceName)
		// the twirp interface does not include "Client" at the end
		clientGoIdent := file.GoImportPath.Ident(interfaceName)
		clientGoIdentString := g.QualifiedGoIdent(clientGoIdent)

		g.P(`type `, structName, ` struct {`)
		g.P(`logger *`, loggerGoIdentString)
		g.P(`client `, clientGoIdentString)
		g.P(`}`)
		g.P()

		for _, method := range service.Methods {
			if err := protogenutil.ValidateMethodUnary(method); err != nil {
				return err
			}
			requestParameterStrings, err := protogenutil.GetParameterStrings(g, method.Input.Fields)
			if err != nil {
				return err
			}
			responseParameterStrings, err := protogenutil.GetParameterStrings(g, method.Output.Fields)
			if err != nil {
				return err
			}
			funcName := method.GoName
			funcParameterStrings := append([]string{`ctx ` + contextGoIdentString}, requestParameterStrings...)
			funcReturnStrings := append(responseParameterStrings, `_ error`)
			if len(funcParameterStrings) > 2 || len(funcReturnStrings) > 2 {
				g.P(method.Comments.Leading, `func (s *`, structName, `) `, funcName, `(`)
				for _, funcParameterString := range funcParameterStrings {
					g.P(funcParameterString, `,`)
				}
				g.P(`) (`, strings.Join(funcReturnStrings, `, `), `) {`)
			} else {
				g.P(method.Comments.Leading, `func (s *`, structName, `) `, funcName, `(`, strings.Join(funcParameterStrings, `, `), `) (`, strings.Join(funcReturnStrings, `, `), `) {`)
			}

			requestGoIdentString := g.QualifiedGoIdent(method.Input.GoIdent)
			if len(funcReturnStrings) == 1 {
				g.P(`_, err := s.client.`, funcName, `(`)
			} else {
				g.P(`response, err := s.client.`, funcName, `(`)
			}
			g.P(`ctx,`)
			g.P(`&`, requestGoIdentString, `{`)
			for _, field := range method.Input.Fields {
				g.P(field.GoName, `: `, protogenutil.GetUnexportGoName(field.GoName), `,`)
			}
			g.P(`},`)
			g.P(`)`)
			g.P(`if err != nil {`)
			errorReturnString, err := protogenutil.GetParameterErrorReturnString(
				g,
				method.Output.Fields,
				`err`,
			)
			if err != nil {
				return err
			}
			g.P(errorReturnString)
			g.P(`}`)
			returnValueStrings := make([]string, len(method.Output.Fields))
			for i, field := range method.Output.Fields {
				returnValueStrings[i] = "response." + field.GoName
			}
			returnValueStrings = append(returnValueStrings, "nil")
			g.P(`return `, strings.Join(returnValueStrings, ", "))
			g.P(`}`)
			g.P()
		}
		g.P()
	}
	return nil
}