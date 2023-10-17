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
	"fmt"
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
