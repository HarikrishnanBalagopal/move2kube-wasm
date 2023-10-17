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
	environmenttypes "github.com/konveyor/move2kube-wasm/types/environment"
	"github.com/sirupsen/logrus"
	"io/fs"
	"path/filepath"
	"reflect"
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

// GetEnvironmentContext returns the context path within the environment
func (e *Environment) GetEnvironmentContext() string {
	return e.Env.GetContext()
}

// GetEnvironmentOutput returns the output path within the environment
func (e *Environment) GetEnvironmentOutput() string {
	return e.CurrEnvOutputBasePath
}
