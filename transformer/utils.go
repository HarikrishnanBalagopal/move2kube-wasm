package transformer

import (
	"bufio"
	"fmt"
	"github.com/konveyor/move2kube-wasm/common"
	plantypes "github.com/konveyor/move2kube-wasm/types/plan"
	transformertypes "github.com/konveyor/move2kube-wasm/types/transformer"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"path/filepath"
	"strings"
)

func getPlanArtifactsFromArtifacts(services map[string][]transformertypes.Artifact, t transformertypes.Transformer) map[string][]plantypes.PlanArtifact {
	planServices := map[string][]plantypes.PlanArtifact{}
	for sn, s := range services {
		for _, st := range s {
			planServices[sn] = append(planServices[sn], plantypes.PlanArtifact{
				TransformerName: t.Name,
				Artifact:        st,
			})
		}
	}
	return planServices
}

func getNamedAndUnNamedServicesLogMessage(services map[string][]plantypes.PlanArtifact) string {
	nnservices := len(services)
	nuntransformers := len(services[""])
	if _, ok := services[""]; ok {
		nuntransformers--
	}
	return fmt.Sprintf("Identified %d named services and %d to-be-named services", nnservices, nuntransformers)
}

func getIgnorePaths(inputPath string) (ignoreDirectories []string, ignoreContents []string) {
	filePaths, err := common.GetFilesByName(inputPath, []string{common.IgnoreFilename}, nil)
	if err != nil {
		logrus.Warnf("failed to fetch .m2kignore files at path '%s' . Error: %q", inputPath, err)
		return ignoreDirectories, ignoreContents
	}
	for _, filePath := range filePaths {
		file, err := os.Open(filePath)
		if err != nil {
			logrus.Warnf("failed to open the .m2kignore file at path '%s' . Error: %q", filePath, err)
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if len(line) == 0 {
				continue
			}
			if strings.HasSuffix(line, "*") {
				line = strings.TrimSuffix(line, "*")
				path := filepath.Join(filepath.Dir(filePath), line)
				ignoreContents = append(ignoreContents, path)
			} else {
				path := filepath.Join(filepath.Dir(filePath), line)
				ignoreDirectories = append(ignoreDirectories, path)
			}
		}
	}
	return ignoreDirectories, ignoreContents
}

func getTransformerConfig(transformerYamlPath string) (transformertypes.Transformer, error) {
	tc := transformertypes.NewTransformer()
	tc.Spec.TransformerYamlPath = transformerYamlPath
	if err := common.ReadMove2KubeYaml(transformerYamlPath, &tc); err != nil {
		return tc, fmt.Errorf("failed to read the transformer metadata from the yaml file at path '%s' . Error: %w", transformerYamlPath, err)
	}
	if tc.Kind != transformertypes.TransformerKind {
		return tc, fmt.Errorf(
			"the file at path '%s' is not a valid cluster metadata. Expected kind: '%s' Actual kind: '%s'",
			transformerYamlPath, transformertypes.TransformerKind, tc.Kind,
		)
	}
	if tc.Labels == nil {
		tc.Labels = map[string]string{}
	}
	tc.Labels[transformertypes.LabelName] = tc.Name
	var err error
	if tc.Spec.OverrideSelector, err = getSelectorFromInterface(tc.Spec.Override); err != nil {
		logrus.Errorf("failed to parse the override selector for the transformer '%s' , Ignoring selector: %+v . Error: %q", tc.Name, tc.Spec.Override, err)
		tc.Spec.OverrideSelector = nil
	}
	if tc.Spec.DependencySelector, err = getSelectorFromInterface(tc.Spec.Dependency); err != nil {
		logrus.Errorf("failed to parse the dependency selector for the transformer '%s' , Ignoring selector: %+v . Error: %q", tc.Name, tc.Spec.Dependency, err)
		tc.Spec.DependencySelector = nil
	}
	// TODO: Add check for consistency between consumes and produces
	return tc, nil
}

func getSelectorFromInterface(sel interface{}) (labels.Selector, error) {
	if sel == nil {
		return nil, nil
	}
	s := metav1.LabelSelector{}
	err := common.GetObjFromInterface(sel, &s)
	if err != nil {
		return nil, fmt.Errorf("failed to get a label selector object from the interface, ignoring override. Error: %w", err)
	}
	if len(s.MatchExpressions) != 0 || len(s.MatchLabels) != 0 {
		return metav1.LabelSelectorAsSelector(&s)
	}
	return nil, nil
}

func getFilteredTransformers(transformerYamlPaths map[string]string, selector labels.Selector, logError bool) map[string]transformertypes.Transformer {
	filteredTransformerConfigs := map[string]transformertypes.Transformer{}
	overrideSelectors := []labels.Selector{}
	for transformerName, transformerYamlPath := range transformerYamlPaths {
		tc, err := getTransformerConfig(transformerYamlPath)
		if err != nil {
			if logError {
				logrus.Errorf("failed to load the YAML file at path '%s' as a Transformer config. Error: %q", transformerYamlPath, err)
			} else {
				logrus.Debugf("failed to load the YAML file at path '%s' as a Transformer config. Error: %q", transformerYamlPath, err)
			}
			continue
		}
		if ot, ok := filteredTransformerConfigs[tc.Name]; ok {
			logrus.Warnf(
				"Found two transformers with the same name '%s' at paths '%s' and '%s' . Ignoring the one at path '%s'",
				tc.Name, ot.Spec.TransformerYamlPath, tc.Spec.TransformerYamlPath, ot.Spec.TransformerYamlPath,
			)
		}
		if !selector.Matches(labels.Set(tc.Labels)) {
			logrus.Debugf("Ignoring the transformer '%s' because its labels don't match the selector", transformerName)
			continue
		}
		if tc.Spec.OverrideSelector != nil {
			overrideSelectors = append(overrideSelectors, tc.Spec.OverrideSelector)
		}
		if _, ok := transformerTypes[tc.Spec.Class]; ok {
			filteredTransformerConfigs[tc.Name] = tc
			continue
		}
		logrus.Errorf("Ignoring the transformer '%s' since the transformer class '%s' was not found", transformerName, tc.Spec.Class)
	}
	transformerConfigs := map[string]transformertypes.Transformer{}
	for transformerName, tc := range filteredTransformerConfigs {
		if ot, ok := transformerConfigs[tc.Name]; ok {
			logrus.Warnf(
				"Found two transformers with the same name '%s' at paths '%s' and '%s' . Ignoring the one at path '%s'",
				tc.Name, ot.Spec.TransformerYamlPath, tc.Spec.TransformerYamlPath, ot.Spec.TransformerYamlPath,
			)
		}
		ignore := false
		for _, overrideSelector := range overrideSelectors {
			if overrideSelector.Matches(labels.Set(tc.Labels)) {
				ignore = true
				break
			}
		}
		if !ignore {
			transformerConfigs[transformerName] = tc
		}
	}
	return transformerConfigs
}
