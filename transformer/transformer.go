/*
 *  Copyright IBM Corporation 2020, 2021
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

package transformer

import (
	"fmt"
	"github.com/konveyor/move2kube-wasm/common"
	"github.com/konveyor/move2kube-wasm/environment"
	"github.com/konveyor/move2kube-wasm/types/transformer/artifacts"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"path/filepath"

	//"fmt"
	//"github.com/konveyor/move2kube-wasm/common"
	plantypes "github.com/konveyor/move2kube-wasm/types/plan"
	"reflect"

	transformertypes "github.com/konveyor/move2kube-wasm/types/transformer"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Transformer interface defines transformer that transforms files and converts it to ir representation
type Transformer interface {
	Init(tc transformertypes.Transformer, env *environment.Environment) (err error)
	// GetConfig returns the transformer config
	GetConfig() (transformertypes.Transformer, *environment.Environment)
	DirectoryDetect(dir string) (services map[string][]transformertypes.Artifact, err error)
	Transform(newArtifacts []transformertypes.Artifact, alreadySeenArtifacts []transformertypes.Artifact) ([]transformertypes.PathMapping, []transformertypes.Artifact, error)
}

var (
	initialized                  = false
	transformerTypes             = map[string]reflect.Type{}
	transformers                 = []Transformer{}
	invokedByDefaultTransformers = []Transformer{}
	transformerMap               = map[string]Transformer{}
)

// GetInitializedTransformersF returns the list of initialized transformers after filtering
func GetInitializedTransformersF(filters labels.Selector) []Transformer {
	filteredTransformers := []Transformer{}
	for _, t := range GetInitializedTransformers() {
		tc, _ := t.GetConfig()
		if tc.ObjectMeta.Labels == nil {
			tc.ObjectMeta.Labels = map[string]string{}
		}
		if !filters.Matches(labels.Set(tc.ObjectMeta.Labels)) {
			continue
		}
		filteredTransformers = append(filteredTransformers, t)
	}
	return filteredTransformers
}

// GetInitializedTransformers returns the list of initialized transformers
func GetInitializedTransformers() []Transformer {
	return transformers
}

// GetServices returns the list of services detected in a directory
func GetServices(projectName string, dir string, transformerSelector *metav1.LabelSelector) (map[string][]plantypes.PlanArtifact, error) {
	logrus.Trace("GetServices start")
	defer logrus.Trace("GetServices end")
	selectedTransformers := transformers
	if transformerSelector != nil {
		filters, err := metav1.LabelSelectorAsSelector(transformerSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the transformer selector %+v . Error: %w", transformerSelector, err)
		}
		selectedTransformers = GetInitializedTransformersF(filters)
	}
	planServices := map[string][]plantypes.PlanArtifact{}
	logrus.Infof("Planning started on the base directory: '%s'", dir)
	logrus.Debugf("selectedTransformers: %+v", selectedTransformers)
	for _, transformer := range selectedTransformers {
		config, env := transformer.GetConfig()
		if err := env.Reset(); err != nil {
			logrus.Errorf("failed to reset the environment for the transformer named '%s' . Error: %q", config.Name, err)
			continue
		}
		if config.Spec.DirectoryDetect.Levels != 1 {
			continue
		}
		logrus.Infof("[%s] Planning", config.Name)
		newServices, err := transformer.DirectoryDetect(env.Encode(dir).(string))
		if err != nil {
			logrus.Errorf("failed to look for services in the directory '%s' using the transformer named '%s' . Error: %q", dir, config.Name, err)
			continue
		}
		newPlanServices := getPlanArtifactsFromArtifacts(*env.Decode(&newServices).(*map[string][]transformertypes.Artifact), config)
		planServices = plantypes.MergeServices(planServices, newPlanServices)
		if len(newPlanServices) > 0 {
			logrus.Infof(getNamedAndUnNamedServicesLogMessage(newPlanServices))
		}
		common.PlanProgressNumBaseDetectTransformers++
		logrus.Infof("[%s] Done", config.Name)
	}
	logrus.Infof("[Base Directory] %s", getNamedAndUnNamedServicesLogMessage(planServices))
	logrus.Infof("Planning finished on the base directory: '%s'", dir)
	logrus.Info("Planning started on its sub directories")
	nservices, err := walkForServices(dir, planServices)
	if err != nil {
		logrus.Errorf("Transformation planning - Directory Walk failed. Error: %q", err)
	} else {
		planServices = nservices
		logrus.Infoln("Planning finished on its sub directories")
	}
	logrus.Infof("[Directory Walk] %s", getNamedAndUnNamedServicesLogMessage(planServices))
	planServices = nameServices(projectName, planServices)
	logrus.Infof("[Named Services] Identified %d named services", len(planServices))
	return planServices, nil
}

func walkForServices(inputPath string, bservices map[string][]plantypes.PlanArtifact) (map[string][]plantypes.PlanArtifact, error) {
	services := bservices
	ignoreDirectories, ignoreContents := getIgnorePaths(inputPath)
	knownServiceDirPaths := []string{}

	err := filepath.WalkDir(inputPath, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			logrus.Warnf("Skipping path %q due to error. Error: %q", path, err)
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		for _, dirRegExp := range common.DefaultIgnoreDirRegexps {
			if dirRegExp.Match([]byte(filepath.Base(path))) {
				return filepath.SkipDir
			}
		}
		if common.IsPresent(knownServiceDirPaths, path) {
			return filepath.SkipDir // TODO: Should we go inside the directory in this case?
		}
		if common.IsPresent(ignoreDirectories, path) {
			if common.IsPresent(ignoreContents, path) {
				return filepath.SkipDir
			}
			return nil
		}
		common.PlanProgressNumDirectories++
		logrus.Debugf("Planning in directory %s", path)
		numfound := 0
		skipThisDir := false
		for _, transformer := range transformers {
			config, env := transformer.GetConfig()
			logrus.Debugf("[%s] Planning in directory %s", config.Name, path)
			if err := env.Reset(); err != nil {
				logrus.Errorf("failed to reset the environment for the transformer %s . Error: %q", config.Name, err)
				continue
			}
			if config.Spec.DirectoryDetect.Levels == 1 || config.Spec.DirectoryDetect.Levels == 0 {
				continue
			}
			newServicesToArtifacts, err := transformer.DirectoryDetect(env.Encode(path).(string))
			if err != nil {
				logrus.Warnf("[%s] directory detect failed. Error: %q", config.Name, err)
				continue
			}
			for _, newServiceArtifacts := range newServicesToArtifacts {
				for _, newServiceArtifact := range newServiceArtifacts {
					knownServiceDirPaths = append(knownServiceDirPaths, newServiceArtifact.Paths[artifacts.ServiceDirPathType]...)
					for _, serviceDirPath := range newServiceArtifact.Paths[artifacts.ServiceDirPathType] {
						if serviceDirPath == path {
							skipThisDir = true
							break
						}
					}
				}
			}
			newPlanServices := getPlanArtifactsFromArtifacts(*env.Decode(&newServicesToArtifacts).(*map[string][]transformertypes.Artifact), config)
			services = plantypes.MergeServices(services, newPlanServices)
			logrus.Debugf("[%s] Done", config.Name)
			numfound += len(newPlanServices)
			if len(newPlanServices) > 0 {
				msg := getNamedAndUnNamedServicesLogMessage(newPlanServices)
				relpath, err := filepath.Rel(inputPath, path)
				if err != nil {
					logrus.Errorf("failed to make the directory %s relative to the input directory %s . Error: %q", path, inputPath, err)
					logrus.Infof("%s in %s", msg, path)
					continue
				}
				logrus.Infof("%s in %s", msg, relpath)
			}
		}
		logrus.Debugf("planning finished for the directory %s and %d services were detected", path, numfound)
		if skipThisDir || common.IsPresent(ignoreContents, path) {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return services, fmt.Errorf("failed to walk through the directory at path %s . Error: %q", inputPath, err)
	}
	return services, nil
}
