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
	"errors"
	"fmt"
	"github.com/konveyor/move2kube-wasm/common"
	"github.com/konveyor/move2kube-wasm/environment"
	containertypes "github.com/konveyor/move2kube-wasm/environment/container"
	"github.com/konveyor/move2kube-wasm/filesystem"
	"github.com/konveyor/move2kube-wasm/transformer/dockerfilegenerator"
	"github.com/konveyor/move2kube-wasm/types"
	environmenttypes "github.com/konveyor/move2kube-wasm/types/environment"
	"github.com/konveyor/move2kube-wasm/types/transformer/artifacts"
	"github.com/spf13/cast"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"path/filepath"
	"runtime"
	"sort"

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

type processType int

const (
	consume processType = iota
	passthrough
	dependency
)

const (
	// ALLOW_ALL_ARTIFACT_TYPES is a wild card that allows a transformer to produce all types of artifacts
	ALLOW_ALL_ARTIFACT_TYPES = "*"
	// DEFAULT_SELECTED_LABEL is a label that can be used to remove a transformer from the list of transformers that are selected by default.
	DEFAULT_SELECTED_LABEL = types.GroupName + "/default-selected"
	// CONTAINER_BASED_LABEL is a label that indicates that the transformer needs to spawn containers to run.
	CONTAINER_BASED_LABEL = types.GroupName + "/container-based"
)

var (
	initialized                  = false
	transformerTypes             = map[string]reflect.Type{}
	transformers                 = []Transformer{}
	invokedByDefaultTransformers = []Transformer{}
	transformerMap               = map[string]Transformer{}
)

func init() {
	transformerObjs := []Transformer{
		//new(external.Starlark),
		//new(external.Executable),
		//
		//new(Router),
		//
		//new(dockerfile.DockerfileDetector),
		//new(dockerfile.DockerfileParser),
		//new(dockerfile.DockerfileImageBuildScript),
		//new(dockerfilegenerator.NodejsDockerfileGenerator),
		new(dockerfilegenerator.GolangDockerfileGenerator),
		//new(dockerfilegenerator.PHPDockerfileGenerator),
		//new(dockerfilegenerator.PythonDockerfileGenerator),
		//new(dockerfilegenerator.RubyDockerfileGenerator),
		//new(dockerfilegenerator.RustDockerfileGenerator),
		//new(dockerfilegenerator.DotNetCoreDockerfileGenerator),
		//new(java.JarAnalyser),
		//new(java.WarAnalyser),
		//new(java.EarAnalyser),
		//new(java.Tomcat),
		//new(java.Liberty),
		//new(java.Jboss),
		//new(java.MavenAnalyser),
		//new(java.GradleAnalyser),
		//new(java.ZuulAnalyser),
		//new(windows.WinConsoleAppDockerfileGenerator),
		//new(windows.WinSilverLightWebAppDockerfileGenerator),
		//new(windows.WinWebAppDockerfileGenerator),
		//new(CNBContainerizer),
		//new(compose.ComposeAnalyser),
		//new(compose.ComposeGenerator),
		//
		//new(CloudFoundry),
		//
		//new(containerimage.ContainerImagesPushScript),
		//
		//new(kubernetes.ClusterSelectorTransformer),
		//new(kubernetes.Kubernetes),
		//new(kubernetes.Knative),
		//new(kubernetes.Tekton),
		//// new(kubernetes.ArgoCD),
		//new(kubernetes.BuildConfig),
		//new(kubernetes.Parameterizer),
		//new(kubernetes.KubernetesVersionChanger),
		//new(kubernetes.OperatorTransformer),
		//
		//new(ReadMeGenerator),
		//new(InvokeDetect),
	}
	transformerTypes = common.GetTypesMap(transformerObjs)
}

// Init initializes the transformers
func Init(assetsPath, sourcePath string, selector labels.Selector, outputPath, projName string) (map[string]string, error) {
	yamlPaths, err := common.GetFilesByExt(assetsPath, []string{".yml", ".yaml"})
	if err != nil {
		return nil, fmt.Errorf("failed to look for yaml files in the directory '%s' . Error: %w", assetsPath, err)
	}
	transformerYamlPaths := map[string]string{}
	for _, yamlPath := range yamlPaths {
		tc, err := getTransformerConfig(yamlPath)
		if err != nil {
			logrus.Debugf("failed to load the transformer config file at path '%s' . Error: %q", yamlPath, err)
			continue
		}
		if otc, ok := transformerYamlPaths[tc.Name]; ok {
			logrus.Warnf("Duplicate transformer configs with same name '%s' found. Ignoring '%s' in favor of '%s'", tc.Name, otc, yamlPath)
		}
		transformerYamlPaths[tc.Name] = yamlPath
	}
	deselectedTransformers, err := InitTransformers(transformerYamlPaths, selector, sourcePath, outputPath, projName, false, false)
	if err != nil {
		return deselectedTransformers, fmt.Errorf(
			"failed to initialize the transformers using the source path '%s' and the output path '%s' . Error: %w",
			sourcePath, outputPath, err,
		)
	}
	return deselectedTransformers, nil
}

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

// Destroy destroys the transformers
func Destroy() {
	for _, t := range transformers {
		_, env := t.GetConfig()
		if err := env.Destroy(); err != nil {
			logrus.Errorf("Unable to destroy environment : %s", err)
		}
	}
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

// InitTransformers initializes a subset of transformers
func InitTransformers(transformerYamlPaths map[string]string, selector labels.Selector, sourcePath, outputPath, projName string, logError, preExistingPlan bool) (map[string]string, error) {
	logrus.Trace("InitTransformers start")
	defer logrus.Trace("InitTransformers end")
	if initialized {
		logrus.Debug("already initialized")
		return nil, nil
	}
	//transformerFilterString := qaengine.FetchStringAnswer(
	//	common.TransformerSelectorKey,
	//	"Specify a Kubernetes style selector to select only the transformers that you want to run.",
	//	[]string{"Leave empty to select everything. This is the default."},
	//	"",
	//	nil,
	//)
	//if transformerFilterString != "" {
	//	if transformerFilter, err := common.ConvertStringSelectorsToSelectors(transformerFilterString); err != nil {
	//		logrus.Errorf("failed to parse the transformer filter string: %s . Error: %q", transformerFilterString, err)
	//	} else {
	//		reqs, _ := transformerFilter.Requirements()
	//		selector = selector.Add(reqs...)
	//	}
	//}
	transformerConfigs := getFilteredTransformers(transformerYamlPaths, selector, logError)
	deselectedTransformers := map[string]string{}
	for transformerName, transformerPath := range transformerYamlPaths {
		if _, ok := transformerConfigs[transformerName]; !ok {
			deselectedTransformers[transformerName] = transformerPath
		}
	}
	transformerNames := []string{}
	transformerNamesSelectedByDefault := []string{}
	for transformerName, t := range transformerConfigs {
		transformerNames = append(transformerNames, transformerName)
		if v, ok := t.ObjectMeta.Labels[DEFAULT_SELECTED_LABEL]; !ok || cast.ToBool(v) {
			transformerNamesSelectedByDefault = append(transformerNamesSelectedByDefault, transformerName)
		}
	}
	sort.Strings(transformerNames)
	selectedTransformerNames := []string{"Golang-Dockerfile"}
	//selectedTransformerNames := qaengine.FetchMultiSelectAnswer(
	//	common.ConfigTransformerTypesKey,
	//	"Select all transformer types that you are interested in:",
	//	[]string{"Services that don't support any of the transformer types you are interested in will be ignored."},
	//	transformerNamesSelectedByDefault,
	//	transformerNames,
	//	nil,
	//)
	for _, transformerName := range transformerNames {
		if !common.IsPresent(selectedTransformerNames, transformerName) {
			deselectedTransformers[transformerName] = transformerYamlPaths[transformerName]
		}
	}
	for _, selectedTransformerName := range selectedTransformerNames {
		transformerConfig, ok := transformerConfigs[selectedTransformerName]
		if !ok {
			logrus.Errorf("failed to find the transformer with the name: '%s'", selectedTransformerName)
			continue
		}
		transformerClass, ok := transformerTypes[transformerConfig.Spec.Class]
		if !ok {
			logrus.Errorf("failed to find the transformer class '%s' . Valid transformer classes are: %+v", transformerConfig.Spec.Class, transformerTypes)
			continue
		}
		transformer := reflect.New(transformerClass).Interface().(Transformer)
		transformerContextPath := filepath.Dir(transformerConfig.Spec.TransformerYamlPath)
		envInfo := environment.EnvInfo{
			Name:            transformerConfig.Name,
			ProjectName:     projName,
			Isolated:        transformerConfig.Spec.Isolated,
			Source:          sourcePath,
			Output:          outputPath,
			Context:         transformerContextPath,
			RelTemplatesDir: transformerConfig.Spec.TemplatesDir,
			EnvPlatformConfig: environmenttypes.EnvPlatformConfig{
				Container: environmenttypes.Container{},
				Platforms: []string{runtime.GOOS},
			},
		}
		for src, dest := range transformerConfig.Spec.ExternalFiles {
			if err := filesystem.Replicate(filepath.Join(transformerContextPath, src), filepath.Join(transformerContextPath, dest)); err != nil {
				logrus.Errorf(
					"failed to copy external files for transformer '%s' from source path '%s' to destination path '%s' . Error: %q",
					transformerConfig.Name, src, dest, err,
				)
			}
		}
		if preExistingPlan {
			if v, ok := transformerConfig.Labels[CONTAINER_BASED_LABEL]; ok && cast.ToBool(v) {
				envInfo.SpawnContainers = true
			}
		}
		env, err := environment.NewEnvironment(envInfo, nil)
		if err != nil {
			return deselectedTransformers, fmt.Errorf("failed to create the environment %+v . Error: %w", envInfo, err)
		}
		if err := transformer.Init(transformerConfig, env); err != nil {
			if errors.Is(err, containertypes.ErrNoContainerRuntime) {
				logrus.Debugf("failed to initialize the transformer '%s' . Error: %q", transformerConfig.Name, err)
			} else {
				logrus.Errorf("failed to initialize the transformer '%s' . Error: %q", transformerConfig.Name, err)
			}
			continue
		}
		transformers = append(transformers, transformer)
		transformerMap[selectedTransformerName] = transformer
		if transformerConfig.Spec.InvokedByDefault.Enabled {
			invokedByDefaultTransformers = append(invokedByDefaultTransformers, transformer)
		}
	}
	initialized = true
	return deselectedTransformers, nil
}
