/*
 *  Copyright IBM Corporation 2021, 2022
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

package common

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/konveyor/move2kube-wasm/types"
	"github.com/mitchellh/mapstructure"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"

	//"github.com/go-git/go-git/v5"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ObjectToYamlBytes encodes an object to yaml
func ObjectToYamlBytes(data interface{}) ([]byte, error) {
	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	encoder.SetIndent(2)
	if err := encoder.Encode(data); err != nil {
		logrus.Errorf("Failed to encode the object to yaml. Error: %q", err)
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		logrus.Errorf("Failed to close the yaml encoder. Error: %q", err)
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteYaml writes encodes object as yaml and writes it to a file
func WriteYaml(outputPath string, data interface{}) error {
	yamlBytes, err := ObjectToYamlBytes(data)
	if err != nil {
		logrus.Errorf("Failed to encode the object as a yaml string. Error: %q", err)
		return err
	}
	return os.WriteFile(outputPath, yamlBytes, DefaultFilePermission)
}

// IsParent can be used to check if a path is one of the parent directories of another path.
// Also returns true if the paths are the same.
func IsParent(child, parent string) bool {
	var err error
	child, err = filepath.Abs(child)
	if err != nil {
		logrus.Fatalf("Failed to make the path %s absolute. Error: %s", child, err)
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		logrus.Fatalf("Failed to make the path %s absolute. Error: %s", parent, err)
	}
	if parent == "/" {
		return true
	}
	childParts := strings.Split(child, string(os.PathSeparator))
	parentParts := strings.Split(parent, string(os.PathSeparator))
	if len(parentParts) > len(childParts) {
		return false
	}
	for i, parentPart := range parentParts {
		if childParts[i] != parentPart {
			return false
		}
	}
	return true
}

// IsPresent checks if a value is present in a slice
func IsPresent[C comparable](list []C, value C) bool {
	for _, val := range list {
		if val == value {
			return true
		}
	}
	return false
}

// CopyFile copies a file from src to dst.
// The dst file will be truncated if it exists.
// Returns an error if it failed to copy all the bytes.
func CopyFile(dst, src string) error {
	srcfile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open the source file at path %q Error: %q", src, err)
	}
	defer srcfile.Close()
	srcfileinfo, err := srcfile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get size of the source file at path %q Error: %q", src, err)
	}
	srcfilesize := srcfileinfo.Size()
	dstfile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcfileinfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create the destination file at path %q Error: %q", dst, err)
	}
	defer dstfile.Close()
	written, err := io.Copy(dstfile, srcfile)
	if written != srcfilesize {
		return fmt.Errorf("failed to copy all the bytes from source %q to destination %q. %d out of %d bytes written. Error: %v", src, dst, written, srcfilesize, err)
	}
	if err != nil {
		return fmt.Errorf("failed to copy from source %q to destination %q. Error: %q", src, dst, err)
	}
	return dstfile.Close()
}

// GetFilesByName returns files by name
func GetFilesByName(inputPath string, names []string, nameRegexes []string) ([]string, error) {
	var files []string
	if info, err := os.Stat(inputPath); os.IsNotExist(err) {
		return files, fmt.Errorf("failed to stat the directory '%s' . Error: %w", inputPath, err)
	} else if !info.IsDir() {
		logrus.Warnf("The path '%s' is not a directory.", inputPath)
	}
	compiledNameRegexes := []*regexp.Regexp{}
	for _, nameRegex := range nameRegexes {
		compiledNameRegex, err := regexp.Compile(nameRegex)
		if err != nil {
			logrus.Errorf("failed to compile the regular expression '%s' . Ignoring. Error: %q", nameRegex, err)
			continue
		}
		compiledNameRegexes = append(compiledNameRegexes, compiledNameRegex)
	}
	err := filepath.WalkDir(inputPath, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			if path == inputPath {
				// if the root directory returns an error then stop walking and return this error
				return err
			}
			logrus.Warnf("Skipping path '%s' due to error: %q", path, err)
			return nil
		}
		// Skip directories
		if info.IsDir() {
			for _, dirRegExp := range DefaultIgnoreDirRegexps {
				if dirRegExp.Match([]byte(filepath.Base(path))) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		fname := filepath.Base(path)
		for _, name := range names {
			if name == fname {
				files = append(files, path)
				return nil
			}
		}
		for _, compiledNameRegex := range compiledNameRegexes {
			if compiledNameRegex.MatchString(fname) {
				files = append(files, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return files, fmt.Errorf("failed to walk through the files in the directory '%s' . Error: %w", inputPath, err)
	}
	logrus.Debugf("found %d files with the names %+v", len(files), names)
	return files, nil
}

// CleanAndFindCommonDirectory finds the common ancestor directory among a list of absolute paths.
// Cleans the paths you give it before finding the directory.
// Also see FindCommonDirectory
func CleanAndFindCommonDirectory(paths []string) string {
	cleanedpaths := make([]string, len(paths))
	for i, path := range paths {
		cleanedpaths[i] = filepath.Clean(path)
	}
	return FindCommonDirectory(cleanedpaths)
}

// FindCommonDirectory finds the common ancestor directory among a list of cleaned absolute paths.
// Will not clean the paths you give it before trying to find the directory.
// Also see CleanAndFindCommonDirectory
func FindCommonDirectory(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	slash := string(filepath.Separator)
	commonDir := paths[0]
	for commonDir != slash {
		found := true
		for _, path := range paths {
			if !strings.HasPrefix(path+slash, commonDir+slash) {
				found = false
				break
			}
		}
		if found {
			break
		}
		commonDir = filepath.Dir(commonDir)
	}
	return commonDir
}

// GatherGitInfo tries to find the git repo for the path if one exists.
//func GatherGitInfo(path string) (repoName, repoDir, repoHostName, repoURL, repoBranch string, err error) {
//	if finfo, err := os.Stat(path); err != nil {
//		return "", "", "", "", "", fmt.Errorf("failed to stat the path '%s' . Error %w", path, err)
//	} else if !finfo.IsDir() {
//		pathDir := filepath.Dir(path)
//		logrus.Debugf("The path '%s' is not a directory. Using the path '%s' instead.", path, pathDir)
//		path = pathDir
//	}
//	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
//	if err != nil {
//		return "", "", "", "", "", fmt.Errorf("failed to open the path '%s' as a git repo. Error: %w", path, err)
//	}
//	workTree, err := repo.Worktree()
//	if err != nil {
//		return "", "", "", "", "", fmt.Errorf("failed to get the repo working tree/directory. Error: %w", err)
//	}
//	repoDir = workTree.Filesystem.Root()
//	ref, err := repo.Head()
//	if err != nil {
//		return "", "", "", "", "", fmt.Errorf("failed to get the current branch. Error: %w", err)
//	}
//	logrus.Debugf("current branch/tag: %#v", ref)
//	repoBranch = filepath.Base(string(ref.Name()))
//	remotes, err := repo.Remotes()
//	if err != nil || len(remotes) == 0 {
//		logrus.Debugf("failed to find any remote repo urls for the repo at path '%s' . Error: %q", path, err)
//		logrus.Debugf("git no remotes case - repoName '%s', repoDir '%s', repoHostName '%s', repoURL '%s', repoBranch '%s'", repoName, repoDir, repoHostName, repoURL, repoBranch)
//		return repoName, repoDir, repoHostName, repoURL, repoBranch, nil
//	}
//	var preferredRemote *git.Remote
//	if preferredRemote = getGitRemoteByName(remotes, "upstream"); preferredRemote == nil {
//		if preferredRemote = getGitRemoteByName(remotes, "origin"); preferredRemote == nil {
//			preferredRemote = remotes[0]
//		}
//	}
//	if len(preferredRemote.Config().URLs) == 0 {
//		err = fmt.Errorf("unable to get origins")
//		logrus.Debugf("%s", err)
//	}
//	u := preferredRemote.Config().URLs[0]
//	repoURL = u
//	if strings.HasPrefix(u, "git@") {
//		// Example: git@github.com:konveyor/move2kube.git
//		withoutGitAt := strings.TrimPrefix(u, "git@")
//		idx := strings.Index(withoutGitAt, ":")
//		if idx < 0 {
//			return "", "", "", "", "", fmt.Errorf("failed to parse the remote host url '%s' as a git ssh url. Error: %w", u, err)
//		}
//		domain := withoutGitAt[:idx]
//		rest := withoutGitAt[idx+1:]
//		newUrl := "https://" + domain + "/" + rest
//		logrus.Debugf("final parsed git ssh url to normal url: '%s'", newUrl)
//		giturl, err := url.Parse(newUrl)
//		if err != nil {
//			return "", "", "", "", "", fmt.Errorf("failed to parse the remote host url '%s' . Error: %w", newUrl, err)
//		}
//		logrus.Debugf("parsed ssh case - giturl: %#v", giturl)
//		repoHostName = giturl.Host
//		repoName = filepath.Base(giturl.Path)
//		repoName = strings.TrimSuffix(repoName, filepath.Ext(repoName))
//		logrus.Debugf("git ssh case - repoName '%s', repoDir '%s', repoHostName '%s', repoURL '%s', repoBranch '%s'", repoName, repoDir, repoHostName, repoURL, repoBranch)
//		return repoName, repoDir, repoHostName, repoURL, repoBranch, nil
//	}
//
//	giturl, err := url.Parse(u)
//	if err != nil {
//		return "", "", "", "", "", fmt.Errorf("failed to parse the remote host url '%s' . Error: %w", u, err)
//	}
//	logrus.Debugf("parsed normal case - giturl: %#v", giturl)
//	repoHostName = giturl.Host
//	repoName = filepath.Base(giturl.Path)
//	repoName = strings.TrimSuffix(repoName, filepath.Ext(repoName))
//	logrus.Debugf("git normal case - repoName '%s', repoDir '%s', repoHostName '%s', repoURL '%s', repoBranch '%s'", repoName, repoDir, repoHostName, repoURL, repoBranch)
//	return repoName, repoDir, repoHostName, repoURL, repoBranch, nil
//}
//
//func getGitRemoteByName(remotes []*git.Remote, remoteName string) *git.Remote {
//	for _, r := range remotes {
//		if r.Config().Name == remoteName {
//			return r
//		}
//	}
//	return nil
//}

// NormalizeForMetadataName converts the string to be compatible for service name
func NormalizeForMetadataName(metadataName string) string {
	if metadataName == "" {
		logrus.Errorf("failed to normalize for service/metadata name because it is an empty string")
		return ""
	}
	newName := disallowedDNSCharactersRegex.ReplaceAllLiteralString(strings.ToLower(metadataName), "-")
	maxLength := 63
	if len(newName) > maxLength {
		newName = newName[0:maxLength]
	}
	newName = ReplaceStartingTerminatingHyphens(newName, "a", "z")
	if newName != metadataName {
		logrus.Infof("Changing metadata name from %s to %s", metadataName, newName)
	}
	return newName
}

// ReplaceStartingTerminatingHyphens replaces the first and last characters of a string if they are hyphens
func ReplaceStartingTerminatingHyphens(str, startReplaceStr, endReplaceStr string) string {
	first := str[0]
	last := str[len(str)-1]
	if first == '-' {
		logrus.Debugf("Warning: The first character of the name %q are not alphanumeric.", str)
		str = startReplaceStr + str[1:]
	}
	if last == '-' {
		logrus.Debugf("Warning: The last character of the name %q are not alphanumeric.", str)
		str = str[:len(str)-1] + endReplaceStr
	}
	return str
}

// StringToK8sQuantityHookFunc returns a DecodeHookFunc that converts strings to a Kubernetes resource limits quantity.
func StringToK8sQuantityHookFunc() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() != reflect.String {
			return data, nil
		}
		if to != reflect.TypeOf(resource.Quantity{}) {
			return data, nil
		}
		quantity, err := resource.ParseQuantity(data.(string))
		if err != nil {
			return data, fmt.Errorf("failed to parse the string '%s' as a K8s Quantity. Error: %w", data.(string), err)
		}
		return quantity, nil
	}
}

// GetObjFromInterface loads from map[string]interface{} to struct
func GetObjFromInterface(obj interface{}, loadinto interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: StringToK8sQuantityHookFunc(),
		Result:     &loadinto,
		TagName:    "yaml",
		Squash:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to get the mapstructure decoder for the type %T . Error: %w", loadinto, err)
	}
	// logrus.Debugf("Loading data into %+v from %+v", loadinto, obj)
	if err := decoder.Decode(obj); err != nil {
		return fmt.Errorf("failed to decode the object of type %T and value %+v into the type %T . Error: %w", obj, obj, loadinto, err)
	}
	// logrus.Debugf("Object Loaded is %+v", loadinto)
	return nil
}

// ReadMove2KubeYaml reads move2kube specific yaml files (like m2k.plan) into an struct.
// It checks if apiVersion to see if the group is move2kube and also reports if the
// version is different from the expected version.
func ReadMove2KubeYaml(path string, out interface{}) error {
	yamlData, err := os.ReadFile(path)
	if err != nil {
		logrus.Errorf("Failed to read the yaml file at path %s Error: %q", path, err)
		return err
	}
	yamlMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(yamlData), yamlMap); err != nil {
		logrus.Debugf("Error occurred while unmarshalling yaml file at path %s Error: %q", path, err)
		return err
	}
	groupVersionI, ok := yamlMap["apiVersion"]
	if !ok {
		err := fmt.Errorf("did not find apiVersion in the yaml file at path %s", path)
		logrus.Debug(err)
		return err
	}
	groupVersionStr, ok := groupVersionI.(string)
	if !ok {
		err := fmt.Errorf("the apiVersion is not a string in the yaml file at path %s", path)
		logrus.Debug(err)
		return err
	}
	groupVersion, err := schema.ParseGroupVersion(groupVersionStr)
	if err != nil {
		logrus.Debugf("Failed to parse the apiVersion %s Error: %q", groupVersionStr, err)
		return err
	}
	if groupVersion.Group != types.SchemeGroupVersion.Group {
		err := fmt.Errorf("the file at path %s doesn't have the correct group. Expected group %s Actual group %s", path, types.SchemeGroupVersion.Group, groupVersion.Group)
		logrus.Debug(err)
		return err
	}
	if groupVersion.Version != types.SchemeGroupVersion.Version {
		logrus.Warnf("The file at path %s was generated using a different version. File version is %s and move2kube version is %s", path, groupVersion.Version, types.SchemeGroupVersion.Version)
	}
	if err := yaml.Unmarshal(yamlData, out); err != nil {
		logrus.Debugf("Error occurred while unmarshalling yaml file at path %s Error: %q", path, err)
		return err
	}
	return nil
}

// GetSHA256Hash returns the SHA256 hash of the string.
// The hash is 256 bits/32 bytes and encoded as a 64 char hexadecimal string.
func GetSHA256Hash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// MakeStringDNSNameCompliantWithoutDots makes the string into a valid DNS name without dots.
func MakeStringDNSNameCompliantWithoutDots(s string) string {
	name := strings.ToLower(s)
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllLiteralString(name, "-")
	start, end := name[0], name[len(name)-1]
	if start == '-' || end == '-' {
		logrus.Debugf("The first and/or last characters of the string %q are not alphanumeric.", s)
	}
	return name
}

// MakeStringDNSLabelNameCompliant makes the string a valid DNS label name.
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
// 1. contain at most 63 characters
// 2. contain only lowercase alphanumeric characters or '-'
// 3. start with an alphanumeric character
// 4. end with an alphanumeric character
func MakeStringDNSLabelNameCompliant(s string) string {
	name := s
	if len(name) > 63 {
		hash := GetSHA256Hash(name)
		hash = hash[:32]
		name = name[:63-33] // leave room for the hash (32 chars) plus hyphen (1 char).
		name = name + "-" + hash
	}
	return MakeStringDNSNameCompliantWithoutDots(name)
}

// MakeStringK8sServiceNameCompliant makes the string a valid K8s service name.
// See https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#rfc-1035-label-names
// 1. contain at most 63 characters
// 2. contain only lowercase alphanumeric characters or '-'
// 3. start with an alphabetic character
// 4. end with an alphanumeric character
func MakeStringK8sServiceNameCompliant(s string) string {
	if strings.TrimSpace(s) == "" {
		logrus.Errorf("empty string given to create k8s service name")
		return s
	}
	if !regexp.MustCompile(`^[a-zA-Z]`).MatchString(s) {
		logrus.Warnf("the given k8s service name '%s' starts with a non-alphabetic character", s)
	}
	return MakeStringDNSLabelNameCompliant(s)
}

// GetTypesMap returns a type registry for the types in the array
func GetTypesMap(typeInstances interface{}) (typesMap map[string]reflect.Type) {
	typesMap = map[string]reflect.Type{}
	types := reflect.ValueOf(typeInstances)
	for i := 0; i < types.Len(); i++ {
		t := reflect.TypeOf(types.Index(i).Interface()).Elem()
		tn := t.Name()
		if ot, ok := typesMap[tn]; ok {
			logrus.Errorf("Two transformer classes have the same name %s : %T, %T; Ignoring %T", tn, ot, t, t)
			continue
		}
		typesMap[tn] = t
	}
	return typesMap
}

// GetFilesByExt returns files by extension
func GetFilesByExt(inputPath string, exts []string) ([]string, error) {
	var files []string
	if info, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat the directory '%s' . Error: %w", inputPath, err)
	} else if !info.IsDir() {
		logrus.Warnf("The path '%s' is not a directory.", inputPath)
	}
	err := filepath.WalkDir(inputPath, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			if path == inputPath {
				// if the root directory returns an error then stop walking and return this error
				return err
			}
			logrus.Warnf("Skipping the path '%s' due to error: %q", path, err)
			return nil
		}
		// Skip directories
		if info.IsDir() {
			for _, dirRegExp := range DefaultIgnoreDirRegexps {
				if dirRegExp.Match([]byte(filepath.Base(path))) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		fext := filepath.Ext(path)
		for _, ext := range exts {
			if fext == ext {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return files, fmt.Errorf("failed to walk through the files in the directory '%s' . Error: %w", inputPath, err)
	}
	logrus.Debugf("found %d files with the extensions %+v", len(files), exts)
	return files, nil
}

// MakeStringEnvNameCompliant makes the string into a valid Environment variable name.
func MakeStringEnvNameCompliant(s string) string {
	name := strings.ToUpper(s)
	name = regexp.MustCompile(`[^A-Z0-9_]`).ReplaceAllLiteralString(name, "_")
	if regexp.MustCompile(`^[0-9]`).Match([]byte(name)) {
		logrus.Debugf("The first characters of the string %q must not be a digit.", s)
	}
	return name
}

// Interrupt creates SIGINT signal
func Interrupt() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		logrus.Fatal(err)
		return err
	}
	if err := p.Signal(os.Interrupt); err != nil {
		logrus.Fatal(err)
		return err
	}
	return nil
}
