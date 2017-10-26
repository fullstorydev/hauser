// Copyright 2017 FullStory, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gorepoman

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type Gitter struct {
	// If empty, this git refers to our base repo.
	// If non-empty, this git refers to a temp repo.
	externalPath string
}

// BaseGit returns a gitter that operates on our "home" git repo; ie the current working directory.
func BaseGit() *Gitter {
	return &Gitter{}
}

// TempGit returns a gitter that operates on a temp git repo specified by path.
func TempGit(path string) *Gitter {
	path = strings.TrimSuffix(path, "/.git")
	return &Gitter{externalPath: path}
}

// CommitHash returns the commit hash of the currently checked-out branch.
func (g *Gitter) CommitHash() (string, error) {
	// Run the git command to get the git hash of the latest version of the repo
	hash, err := g.Command("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	// Return the hash after trimming the trailing newline
	return strings.TrimSpace(hash), nil
}

// IsLocallyModified returns turn if a path has local modifications (e.g. uncommitted changes).
func (g *Gitter) IsLocallyModified(path string) (bool, error) {
	if path == "" {
		path = "."
	}
	status, err := g.Command("status", "--porcelain=v1", "--", path)
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// Checkout out the given repo at to the given branch name or commit SHA.
func (g *Gitter) Checkout(branch string) error {
	if g.externalPath == "" {
		panic("cowardly refusing to run git checkout on the base repo")
	}

	if _, err := g.Command("checkout", branch); err != nil {
		return ErrWithMessagef(err, "Failed to checkout git branch %s", branch)
	}
	return nil
}

// MergeBase returns the common ancestor (if any) between two commits.
func (g *Gitter) MergeBase(commitA string, commitB string) (string, error) {
	out, err := g.Command("merge-base", commitA, commitB)
	if err != nil {
		return "", err
	}
	// Trim the newline off the output of the command
	return strings.TrimSpace(out), nil
}

var lsTreeRegex = regexp.MustCompile(`^[0-7]+\s+tree\s+([0-9a-f]+)\s+\S+$`)

// TreeHash returns the tree SHA for the given path in this repo. This is used for content comparisons.
// Pass an empty pkgPath to get the tree SHA for the root (ie, the commit's content sha).
func (g *Gitter) TreeHash(gitRev string, pkgPath string) (string, error) {
	if pkgPath == "" {
		// For the root path, we use `git show <rev> -s --format=%T`
		out, err := g.Command("show", gitRev, "-s", "--format=%T")
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(out), nil
	} else {
		// for subfolders, we use git ls-tree <rev> -- <path>
		output, err := g.Command("ls-tree", gitRev, "--", pkgPath)
		if err != nil {
			return "", err
		}

		output = strings.TrimSpace(output)
		if !lsTreeRegex.MatchString(output) {
			return "", errors.New("git ls-tree output did not match regex: " + output)
		}

		return lsTreeRegex.FindStringSubmatch(output)[1], nil
	}
}

var lsRemoteRegex = regexp.MustCompile(`^([0-9a-f]+)\s+HEAD$`)

// LsRemote returns the server-side commit SHA of the default branch (e.g. usually master) for
// the given git remote url. Used for staleness checks.
func (g *Gitter) LsRemote(remoteUrl string) (string, error) {
	out, err := g.Command("ls-remote", remoteUrl, "HEAD")
	if err != nil {
		return "", err
	}

	out = strings.TrimSpace(out)
	if !lsRemoteRegex.MatchString(out) {
		return "", errors.New("git ls-remote output did not match regex: " + out)
	}

	return lsRemoteRegex.FindStringSubmatch(out)[1], nil
}

var remoteRegex = regexp.MustCompile(`^origin\s+(\S+)\s+\(fetch\)$`)

// Remote returns the URL of the `origin` remote.
func (g *Gitter) Remote() (string, error) {
	output, err := g.Command("remote", "-v")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "(fetch)") {
			if !remoteRegex.MatchString(line) {
				return "", errors.New("git remote output did not match regex: " + line)
			}

			return remoteRegex.FindStringSubmatch(line)[1], nil
		}
	}

	return "", errors.New("git remote output did not contain an origin fetch line: " + output)
}

// Command executs a git command with the given arguments and returns either the combined output,
// or an error if the command exited with non-zero status.
func (g *Gitter) Command(params ...string) (string, error) {
	if g.externalPath != "" {
		params = append([]string{"-C", g.externalPath}, params...)
	}

	// Run the git command as if we were in the top of the repository
	out, err := execBinary("git", nil, params...)
	if err != nil {
		return "", ErrWithMessagef(err, "Failed to execute 'git %s'", strings.Join(params, " "))
	}

	// Return the string-ified version of the output
	return out, nil
}
