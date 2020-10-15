// Copyright 2020 Buf Technologies, Inc.
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

package protoc

import (
	"context"
	"fmt"
	"strings"

	"github.com/bufbuild/buf/internal/buf/bufcore/bufimage"
	"github.com/bufbuild/buf/internal/pkg/app"
	"github.com/bufbuild/buf/internal/pkg/app/appproto/appprotoos"
	"go.uber.org/zap"
)

type pluginInfo struct {
	// Required
	Out string
	// optional
	Opt []string
	// optional
	Path string
}

func newPluginInfo() *pluginInfo {
	return &pluginInfo{}
}

func executePlugin(
	ctx context.Context,
	logger *zap.Logger,
	container app.EnvStderrContainer,
	images []bufimage.Image,
	pluginName string,
	pluginInfo *pluginInfo,
) error {
	if err := appprotoos.NewGenerator(logger).Generate(
		ctx,
		container,
		pluginName,
		pluginInfo.Out,
		bufimage.ImagesToCodeGeneratorRequests(
			images,
			strings.Join(pluginInfo.Opt, ","),
		),
		appprotoos.GenerateWithPluginPath(pluginInfo.Path),
	); err != nil {
		return fmt.Errorf("--%s_out: %v", pluginName, err)
	}
	return nil
}
