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
	manifestLocation = flag.String("d", "",
		"`directory` that contains the manifest file to manipulate. If not set,\n" +
		"    	defaults to GOREPOMAN_ROOT environment variable. If that is not set\n" +
		"    	either, then GOPATH/src will be used.")

	// TODO: make a flag for specifying a branch name, to use with the 'update'
	// command (maybe 'fetch' and 'reconcile' commands, too?)
)

// NOTES
// Currently we could not work properly if we pull down a package and its dependencies and at least one uses hg/svn but at least one uses git.
// --We will see a git, and continue on, but not copy over any of the packages in other version control

func main() {
	// Define the usage of the command line utility
	flag.Usage = func() {
		os.Stderr.WriteString(
`Manage copies of external, upstream go dependencies/repositories.

gorepoman fetch <dep> [init]            - Download and install the specified dep
gorepoman update <dep>                  - Attempt to freshen the specified dep
gorepoman delete <dep>                  - Delete the specified dep
gorepoman list                          - List all managed deps
gorepoman list stale                    - List all out-of-date deps
gorepoman list changed                  - List all deps with local changes
gorepoman reconcile <dep> [cancel|done] - Reconcile a locally-changed dep with
                                          the upstream repo

<dep> is a Go package, e.g. 'github.com/pkg/errors'")

When fetching a dep into a directory in which gorepoman has never been used
before, use the init option. This will create the initial version of the
gorepomanifest.json file. Otherwise, fetch requires the directory already
contain a gorepomanifest.json file.

All other operations require the directory have a gorepomanifest.json file.

Flags must be passed before the action name. For example:
    gorepoman -d some/package/vendor list
Passing them after the action name will NOT work:
    gorepoman list -d some/package/vendor

Flags:`)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}

	// Parse command line arguments
	flag.Parse()

	args := flag.Args()

	// If no arguments were passed, display the help doc
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	action, args := args[0], args[1:]
	var handler handlerFunc
	switch strings.ToLower(action) {
	case "list":
		handler = listRepos
	case "fetch":
		handler = fetchRepo
	case "update":
		handler = updateRepo
	case "delete":
		handler = deleteRepo
	case "reconcile":
		handler = reconcileRepo
	case "help":
		flag.Usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unrecognized action %s - use 'gorepoman help' to see usage", action)
		os.Exit(1)
	}

	// Create a temporary directory for us to work out of; we will want one no matter the action
	tempDir := filepath.Join(os.TempDir(), "gorepoman")
	os.RemoveAll(tempDir)
	if err := os.Mkdir(tempDir, 0777); err != nil {
		gorepoman.PrintError(os.Stderr, errors.Wrap(err, "Failed to create temporary working directory"))
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	srcDir, err := determineManifestLocation()
	if err != nil {
		gorepoman.PrintError(os.Stderr, err)
		os.Exit(1)
	}

	if err := handler(srcDir, tempDir, args); err != nil {
		gorepoman.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}

// handlerFunc implements a single action supported by gorepoman (e.g. "list", "update", etc).
// It should first validate args, then construct a manifest, then perform the action.
type handlerFunc func(srcDir, tempDir string, args []string) error

func determineManifestLocation() (string, error) {
	loc := os.Getenv("GOREPOMAN_ROOT")
	if *manifestLocation != "" {
		loc = *manifestLocation
	} else if loc == "" {
		goPath := os.Getenv("GOPATH")
		if len(goPath) == 0 {
			return "", fmt.Errorf("failed to determine default manifest directory: GOPATH is not set")
		}
		goPaths := strings.Split(goPath, string(os.PathListSeparator))
		loc = filepath.Join(goPaths[0], "src")
	}

	loc, err := filepath.Abs(loc)
	if err != nil {
		return "", fmt.Errorf("failed to determine absolute path of manifest directory: %s", err)
	}
	loc = filepath.Clean(loc)

	fi, err := os.Stat(loc)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("manifest location does not exist: %s", loc)
		} else {
			return "", fmt.Errorf("failed to stat manifest location %s: %s", loc, err)
		}
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("manifest location %s must be a directory", loc)
	}

	if filepath.Base(loc) != "vendor" && !isGoPathSrcDir(loc) {
		return "", fmt.Errorf("manifest location must be named 'vendor' or be the 'src' sub-directory of GOPATH")
	}

	if *manifestLocation == "" {
		// No location was specified, so we decided on one based on environment. So
		// let's be helpful to the user and show them the directory we came up with.
		fmt.Printf("Using manifest directory %s\n", loc)
	}

	return loc, nil
}

func isGoPathSrcDir(loc string) bool {
	if filepath.Base(loc) != "src" {
		return false
	}
	dirStat, err := os.Stat(filepath.Dir(loc))
	if err != nil {
		return false
	}
	goPath := os.Getenv("GOPATH")
	for _, path := range strings.Split(goPath, string(os.PathListSeparator)) {
		pathStat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if os.SameFile(dirStat, pathStat) {
			return true
		}
	}
	return false
}

func fetchRepo(srcDir, tempDir string, args []string) error {
	pkg, args, err := requirePackage(args)
	if err != nil {
		return err
	}
	option, err := getOption(args, "init")
	if err != nil {
		return err
	}

	var manifest *gorepoman.RepoManifest
	if option == "init" {
		manifest, err = gorepoman.CreateManifest(srcDir, tempDir)
	} else {
		manifest, err = gorepoman.ReadManifest(srcDir, tempDir)
	}
	if err != nil {
		return err
	}

	return manifest.PkgFetch(pkg)
}

func listRepos(srcDir, tempDir string, args []string) error {
	option, err := getOption(args, "stale", "changed")
	if err != nil {
		return err
	}

	manifest, err := gorepoman.ReadManifest(srcDir, tempDir)
	if err != nil {
		return err
	}

	switch option {
	case "stale":
		return manifest.PkgListStale()
	case "changed":
		return manifest.PkgListChanged()
	default:
		return manifest.PkgList()
	}
}

func updateRepo(srcDir, tempDir string, args []string) error {
	pkg, args, err := requirePackage(args)
	if err != nil {
		return err
	}
	if _, err = getOption(args); err != nil {
		return err
	}

	manifest, err := gorepoman.ReadManifest(srcDir, tempDir)
	if err != nil {
		return err
	}
	if err := checkPackageExistsInManifest(manifest, pkg); err != nil {
		return err
	}

	return manifest.PkgUpdate(pkg)
}

func deleteRepo(srcDir, tempDir string, args []string) error {
	pkg, args, err := requirePackage(args)
	if err != nil {
		return err
	}
	if _, err = getOption(args); err != nil {
		return err
	}

	manifest, err := gorepoman.ReadManifest(srcDir, tempDir)
	if err != nil {
		return err
	}
	if err := checkPackageExistsInManifest(manifest, pkg); err != nil {
		return err
	}

	return manifest.PkgDelete(pkg, true)
}

func reconcileRepo(srcDir, tempDir string, args []string) error {
	pkg, args, err := requirePackage(args)
	if err != nil {
		return err
	}
	if len(args) > 0 {
		opt := strings.ToLower(args[0])
		if (pkg == "cancel" || pkg == "done") && opt != "cancel" && opt != "done" {
			// user specified package and option backwards; we'll allow it
			pkg, args[0] = args[0], pkg
		}
	}
	option, err := getOption(args, "cancel", "done")
	if err != nil {
		return err
	}

	manifest, err := gorepoman.ReadManifest(srcDir, tempDir)
	if err != nil {
		return err
	}
	if err := checkPackageExistsInManifest(manifest, pkg); err != nil {
		return err
	}

	switch option {
	case "cancel":
		return manifest.PkgReconcileCancel(pkg)
	case "done":
		return manifest.PkgReconcileDone(pkg)
	default:
		return manifest.PkgReconcile(pkg)
	}
}

func requirePackage(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("You must specify a package.")
	}
	pkg, newArgs := args[0], args[1:]
	pkg = strings.TrimRight(pkg, "/")
	return pkg, newArgs, nil
}

func getOption(args []string, allowedOptions ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(allowedOptions) == 0 {
		return "", fmt.Errorf("Unrecognized arguments: %v", strings.Join(args, " "))
	}
	if len(args) == 1 {
		// see if it's an allowed option
		for _, opt := range allowedOptions {
			if opt == args[0] {
				return args[0], nil
			}
		}
	}
	for i, o := range allowedOptions {
		allowedOptions[i] = fmt.Sprintf("%q", o)
	}
	return "", fmt.Errorf("Unrecognized arguments: %v\nExpecting %s or none", strings.Join(args, " "), strings.Join(allowedOptions, ", "))
}

func checkPackageExistsInManifest(manifest *gorepoman.RepoManifest, pkg string) error {
	if !manifest.HasRepository(pkg) {
		maybe := manifest.LikelyMatch(pkg)
		if maybe == "" {
			return fmt.Errorf(`The package %s was not found in the manifest file.
Was it ever checked-in in the first place?`, pkg)
		} else {
			return fmt.Errorf(`The package %s was not found in the manifest file.
Did you mean %s (which is present in manifest)?`, pkg, maybe)
		}
	}
	return nil
}
