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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fullstorydev/gorepoman"
	"github.com/pkg/errors"
)

var (
	srcDir  string
	tempDir = filepath.Join(os.TempDir(), "gorepoman")
)

// NOTES
// Currently we could not work properly if we pull down a package and its dependencies and at least one uses hg/svn but at least one uses git.
// --We will see a git, and continue on, but not copy over any of the packages in other version control

func main() {
	// Define the usage of the command line utility
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "manage copies of external, upstream go dependencies/repositories")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "gorepoman fetch <dep>                   - Download and install the specified dep")
		fmt.Fprintln(os.Stderr, "gorepoman update <dep>                  - Attempt to freshen the specified dep")
		fmt.Fprintln(os.Stderr, "gorepoman delete <dep>                  - Delete the specified dep")
		fmt.Fprintln(os.Stderr, "gorepoman list                          - List all managed deps")
		fmt.Fprintln(os.Stderr, "gorepoman list stale                    - List all out-of-date deps")
		fmt.Fprintln(os.Stderr, "gorepoman list changed                  - List all deps with local changes")
		fmt.Fprintln(os.Stderr, "gorepoman reconcile <dep> [cancel|done] - (advanced) Reconcile a locally-changed dep with the upstream")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "<dep> is e.g. 'github.com/pkg/errors'")
	}

	// Parse command line arguments
	flag.Parse()

	// If no arguments were passed, display the help doc
	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		gorepoman.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}

// Wrapper for the code that we want to run, so that we can be sure we always gracefully exit and clean up temp
func run() (err error) {
	// Sanity check the system setup.
	// 1) We should be in a git repo.
	out, err := gorepoman.BaseGit().Command("rev-parse", "--show-toplevel")
	if err != nil {
		return errors.WithMessage(err, "You do not appear to be inside of a git repo, this tool is meant to be used in a git repo")
	}
	gitRepoStr := strings.TrimSpace(out)
	gitRepoPath, err := filepath.Abs(gitRepoStr)
	if err != nil {
		return errors.Wrapf(err, "Could not resolve current git repo %q into an absolute path", gitRepoStr)
	}

	// 2) GOPATH should be defined, and it should be a subdirectory of the git repo.
	goPaths := filepath.SplitList(os.Getenv("GOPATH"))
	if len(goPaths) == 0 {
		return errors.New("GOPATH is not defined, please define it")
	}
	goPathStr := goPaths[0]
	goPath, err := filepath.Abs(goPathStr)
	if err != nil {
		return errors.Wrapf(err, "Could not resolve first GOPATH etnry %q into an absolute path", goPathStr)
	}

	if !strings.HasPrefix(goPath, gitRepoPath) {
		return errors.Errorf("GOPATH=%q is not a subdirectory of the current git repo %q; see README.md", goPath, gitRepoPath)
	}

	if !gorepoman.Exists(goPath) {
		return errors.Errorf("GOPATH=%q does not exist; please fix", goPath)
	}

	// Check to make sure that the source folder exists
	srcDir = filepath.Join(goPath, "src")
	if !gorepoman.Exists(srcDir) {
		if err := os.Mkdir(srcDir, 0777); err != nil {
			return gorepoman.ErrWithMessagef(err, "directory %q does not exist, and we couldn't create it!", srcDir)
		}
	}

	// Create a temporary directory for us to work out of, we will want one no matter the action
	os.RemoveAll(tempDir)
	if err := os.Mkdir(tempDir, 0777); err != nil {
		return errors.Wrap(err, "Failed to create temporary working directory")
	}
	defer os.RemoveAll(tempDir)

	// Read in the package manifest
	manifest, err := gorepoman.ReadManifest(srcDir, tempDir)
	if err != nil {
		return err
	}

	// ---------- FINISH SETUP -----------

	// Lets go to work!
	return actionHandler(manifest, flag.Args())
}

func setOf(vals ...string) map[string]bool {
	ret := map[string]bool{}
	for _, v := range vals {
		ret[v] = true
	}
	return ret
}

// General method to handle determining which action needs to be preformed
func actionHandler(manifest *gorepoman.RepoManifest, args []string) error {
	// Normalize the case of the action
	action := strings.ToLower(args[0])

	// List of actions we accept at the CLI
	actions := setOf("fetch", "list", "update", "reconcile", "delete", "help")
	options := setOf("done", "cancel", "stale", "changed")

	// Confirm that the specified action is one we support
	if !actions[action] {
		return errors.Errorf("Unrecognized action %s - use 'gorepoman help' to see usage", action)
	}

	// Handle the help action
	if action == "help" {
		flag.Usage()
		return nil
	}

	// Handle the list action
	if action == "list" {
		if len(args) == 1 {
			return manifest.PkgList()
		} else if args[1] == "stale" {
			return manifest.PkgListStale()
		} else if args[1] == "changed" {
			return manifest.PkgListChanged()
		} else {
			return errors.Errorf("Unrecognized argument to list: %s - Expecting stale or changed", args[1])
		}
	}

	// All of the other actions require at least one more parameter (the package to work with)
	if len(args) == 1 {
		return errors.Errorf("Too few arguments for action %s - please specify a package", action)
	}

	// Trim leading and trailing slashes from provided package to put it in the standard format
	pkg := strings.Trim(args[1], "/")

	// If they have the options out of order
	if options[pkg] || actions[pkg] {
		fmt.Printf("You are trying to use the flag '%s' as a package name\n", pkg)
		fmt.Println("Check the order of your parameters")
		fmt.Println("")
		flag.Usage()
		return nil
	}

	// Handle the fetch action
	if action == "fetch" {
		return manifest.PkgFetch(pkg)
	}

	// If the package isn't in our repo, let the user know
	if !manifest.HasRepository(pkg) {
		return errors.Errorf(`The package %s was not found in the manifest file
Was it ever checked in in the first place?
Or, the package you are looking for is actually a subpackage, and you need the name of the parent package`, pkg)
	}

	// Handle the rest of the actions
	switch action {
	case "update":
		return manifest.PkgUpdate(pkg)
	case "reconcile":
		return manifest.PkgReconcile(pkg, args)
	case "delete":
		return manifest.PkgDelete(pkg, true)
	}

	return nil
}
