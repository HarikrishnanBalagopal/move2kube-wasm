package transformer

import (
	"bufio"
	"fmt"
	"github.com/konveyor/move2kube-wasm/common"
	plantypes "github.com/konveyor/move2kube-wasm/types/plan"
	transformertypes "github.com/konveyor/move2kube-wasm/types/transformer"
	"github.com/sirupsen/logrus"
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
