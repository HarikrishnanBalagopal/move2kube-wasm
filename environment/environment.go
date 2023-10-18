/*
 *  Copyright IBM Corporation 2021
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package environment

import (
	"fmt"
	"github.com/konveyor/move2kube-wasm/common"
	"github.com/konveyor/move2kube-wasm/common/deepcopy"
	"github.com/konveyor/move2kube-wasm/common/pathconverters"
	"github.com/konveyor/move2kube-wasm/types"
	environmenttypes "github.com/konveyor/move2kube-wasm/types/environment"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const (
	workspaceDir    = "workspace"
	templatePattern = "{{"
)

var (
	// GRPCEnvName represents the environment variable name used to pass the GRPC server information to the transformers
	GRPCEnvName = strings.ToUpper(types.AppNameShort) + "_QA_GRPC_SERVER"
	// ProjectNameEnvName stores the project name
	ProjectNameEnvName = strings.ToUpper(types.AppNameShort) + "_PROJECT_NAME"
	// SourceEnvName stores the source path
	SourceEnvName = strings.ToUpper(types.AppNameShort) + "_SOURCE"
	// OutputEnvName stores the output path
	OutputEnvName = strings.ToUpper(types.AppNameShort) + "_OUTPUT"
	// ContextEnvName stores the context
	ContextEnvName = strings.ToUpper(types.AppNameShort) + "_CONTEXT"
	// CurrOutputEnvName stores the location of output from the previous iteration
	CurrOutputEnvName = strings.ToUpper(types.AppNameShort) + "_CURRENT_OUTPUT"
	// RelTemplatesDirEnvName stores the rel templates directory
	RelTemplatesDirEnvName = strings.ToUpper(types.AppNameShort) + "_RELATIVE_TEMPLATES_DIR"
	// TempPathEnvName stores the temp path
	TempPathEnvName = strings.ToUpper(types.AppNameShort) + "_TEMP"
	// EnvNameEnvName stores the environment name
	EnvNameEnvName = strings.ToUpper(types.AppNameShort) + "_ENV_NAME"
)

// Environment is used to manage EnvironmentInstances
type Environment struct {
	EnvInfo
	Env          EnvironmentInstance
	Children     []*Environment
	TempPathsMap map[string]string
	active       bool
}

// GetEnvironmentSource returns the source path within the environment
func (e *Environment) GetEnvironmentSource() string {
	return e.Env.GetSource()
}

// EnvironmentInstance represents a actual instance of an environment which the Environment manages
type EnvironmentInstance interface {
	Reset() error
	Stat(name string) (fs.FileInfo, error)
	Download(envpath string) (outpath string, err error)
	Upload(outpath string) (envpath string, err error)
	Exec(cmd environmenttypes.Command, envList []string) (stdout string, stderr string, exitcode int, err error)
	Destroy() error

	GetSource() string
	GetContext() string
}

// Reset resets an environment
func (e *Environment) Reset() error {
	if !e.active {
		logrus.Debug("environment not active. Process is terminating")
		return nil
	}
	e.CurrEnvOutputBasePath = ""
	return e.Env.Reset()
}

// Destroy destroys all artifacts specific to the environment
func (e *Environment) Destroy() error {
	e.active = false
	e.Env.Destroy()
	for _, env := range e.Children {
		if err := env.Destroy(); err != nil {
			logrus.Errorf("Unable to destroy environment : %s", err)
		}
	}
	return nil
}

// Encode encodes all paths in the obj to be relevant to the environment
func (e *Environment) Encode(obj interface{}) interface{} {
	dupobj := deepcopy.DeepCopy(obj)
	if !e.active {
		logrus.Debug("environment not active. Process is terminating")
		return dupobj
	}
	processPath := func(path string) (string, error) {
		if path == "" {
			return path, nil
		}
		if !filepath.IsAbs(path) {
			var err error
			if e.CurrEnvOutputBasePath == "" {
				e.CurrEnvOutputBasePath, err = e.Env.Upload(e.Output)
			}
			return filepath.Join(e.CurrEnvOutputBasePath, path), err
		}
		if common.IsParent(path, e.Source) {
			rel, err := filepath.Rel(e.Source, path)
			if err != nil {
				logrus.Errorf("Unable to make path (%s) relative to source (%s) : %s ", path, e.Source, err)
				return path, err
			}
			return filepath.Join(e.Env.GetSource(), rel), nil
		}
		if common.IsParent(path, e.Context) {
			rel, err := filepath.Rel(e.Context, path)
			if err != nil {
				logrus.Errorf("Unable to make path (%s) relative to source (%s) : %s ", path, e.Source, err)
				return path, err
			}
			return filepath.Join(e.Env.GetContext(), rel), nil
		}
		if !common.IsParent(filepath.Clean(path), common.TempPath) {
			err := fmt.Errorf("path %s points to a unknown path. Removing the path", path)
			logrus.Error(err)
			return "", err
		}
		return e.Env.Upload(path)
	}
	if reflect.ValueOf(obj).Kind() == reflect.String {
		val, err := processPath(obj.(string))
		if err != nil {
			logrus.Errorf("Unable to process paths for obj %+v : %s", obj, err)
		}
		return val
	}
	if err := pathconverters.ProcessPaths(dupobj, processPath); err != nil {
		logrus.Errorf("Unable to process paths for obj %+v : %s", dupobj, err)
	}
	return dupobj
}

// Decode decodes all paths in the passed obj
func (e *Environment) Decode(obj interface{}) interface{} {
	dupobj := deepcopy.DeepCopy(obj)
	if !e.active {
		logrus.Debug("environment not active. Process is terminating")
		return dupobj
	}
	processPath := func(path string) (string, error) {
		if path == "" {
			return path, nil
		}
		if !filepath.IsAbs(path) {
			err := fmt.Errorf("the input path %q is not an absolute path", path)
			logrus.Errorf("%s", err)
			return path, err
		}
		if common.IsParent(path, e.GetEnvironmentSource()) {
			rel, err := filepath.Rel(e.GetEnvironmentSource(), path)
			if err != nil {
				logrus.Errorf("Unable to make path (%s) relative to source (%s) : %s ", path, e.GetEnvironmentSource(), err)
				return path, err
			}
			return filepath.Join(e.Source, rel), nil
		}
		if common.IsParent(path, e.GetEnvironmentContext()) {
			rel, err := filepath.Rel(e.GetEnvironmentContext(), path)
			if err != nil {
				logrus.Errorf("Unable to make path (%s) relative to context (%s) : %s ", path, e.GetEnvironmentContext(), err)
				return path, err
			}
			return filepath.Join(e.Context, rel), nil
		}
		if common.IsParent(path, e.GetEnvironmentOutput()) {
			rel, err := filepath.Rel(e.GetEnvironmentOutput(), path)
			if err != nil {
				logrus.Errorf("Unable to make path (%s) relative to output (%s) : %s ", path, e.GetEnvironmentOutput(), err)
				return path, err
			}
			return rel, nil
		}
		if !common.IsParent(filepath.Clean(path), common.TempPath) {
			err := fmt.Errorf("path %s points to a unknown path. Removing the path", path)
			logrus.Error(err)
			return "", err
		}
		return path, nil
	}
	if reflect.ValueOf(dupobj).Kind() == reflect.String {
		val, err := processPath(dupobj.(string))
		if err != nil {
			logrus.Errorf("Unable to process paths for obj %+v : %s", obj, err)
		}
		return val
	}
	if err := pathconverters.ProcessPaths(dupobj, processPath); err != nil {
		logrus.Errorf("Unable to process paths for obj %+v : %s", dupobj, err)
	}
	return dupobj
}

// NewEnvironment creates a new environment
func NewEnvironment(envInfo EnvInfo, grpcQAReceiver net.Addr) (env *Environment, err error) {
	if !common.IsPresent(envInfo.EnvPlatformConfig.Platforms, runtime.GOOS) && envInfo.EnvPlatformConfig.Container.Image == "" {
		return nil, fmt.Errorf("platform '%s' is not supported", runtime.GOOS)
	}
	containerInfo := envInfo.EnvPlatformConfig.Container
	tempPath, err := os.MkdirTemp(common.TempPath, "environment-"+envInfo.Name+"-*")
	if err != nil {
		return env, fmt.Errorf("failed to create the temporary directory. Error: %w", err)
	}
	envInfo.TempPath = tempPath
	env = &Environment{
		EnvInfo:      envInfo,
		Children:     []*Environment{},
		TempPathsMap: map[string]string{},
		active:       true,
	}
	if containerInfo.Image == "" {
		env.Env, err = NewLocal(envInfo, grpcQAReceiver)
		if err != nil {
			return env, fmt.Errorf("failed to create the local environment. Error: %w", err)
		}
		return env, nil
	}
	envVariableName := common.MakeStringEnvNameCompliant(containerInfo.Image)
	// TODO: replace below signalling mechanism with `prefersLocalExecutuion: true` in the transformer.yaml
	// Check if image is part of the current environment.
	// It will be set as environment variable with root as base path of move2kube
	// When running in a process shared environment the environment variable will point to the base pid of the container for the image
	envvars := os.Environ()
	found := ""
	for _, envvar := range envvars {
		envvarpair := strings.SplitN(envvar, "=", 2)
		if len(envvarpair) == 2 && envvarpair[0] == envVariableName {
			found = envvarpair[1]
			break
		}
	}
	if found == "" {
		logrus.Debugf("did not find the environment variable '%s'", envVariableName)
	} else {
		logrus.Debugf("found the environment variable '%s' with the value '%s'", envVariableName, found)
		if _, err := cast.ToIntE(found); err != nil {
			// the value is a string, probably the path to the transformer folder inside the image.
			envInfo.Context = found
			env.Env, err = NewLocal(envInfo, grpcQAReceiver)
			if err == nil {
				return env, nil
			}
			logrus.Errorf("failed to create the local environment. Falling back to peer container environment. Error: %q", err)
		}
	}
	//if env.Env == nil {
	//	env.Env, err = NewPeerContainer(envInfo, grpcQAReceiver, containerInfo, envInfo.SpawnContainers)
	//	if err != nil {
	//		return env, fmt.Errorf("failed to create the peer container environment. Error: %w", err)
	//	}
	//}
	return env, nil
}

// GetEnvironmentContext returns the context path within the environment
func (e *Environment) GetEnvironmentContext() string {
	return e.Env.GetContext()
}

// GetEnvironmentOutput returns the output path within the environment
func (e *Environment) GetEnvironmentOutput() string {
	return e.CurrEnvOutputBasePath
}
