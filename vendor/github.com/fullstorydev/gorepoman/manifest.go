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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

const (
	manifestFile   = "gorepomanifest.json"
	directoryPerm  = 0777
	filePerm       = 0666
	failureWarning = "Use git to roll back changes to go/src, manifest is most likely out of sync now."
)

var (
	exportDir = filepath.Join(assertEnv("HOME"), ".gorepoman", "src")
)

// Defines how we store data in the manifest JSON file and the manifest object once it is loaded
type RepoManifest struct {
	Repositories RepoList
	Stale        bool
	srcDir       string
	tmpDir       string
}

type RepoList map[string]PackageData

type PackageData struct {
	GitRemote      string `json:"git-remote"`
	TrackingBranch string `json:"git-remote-branch,omitempty"`
	CurrentGitHash string `json:"current-version-git-hash"`
	ContentHash    string `json:"content-hash"`
}

func (pd PackageData) TrackingRef() string {
	if pd.TrackingBranch == "" {
		return "HEAD"
	}
	return pd.TrackingBranch
}

func (manifest *RepoManifest) Repository(packageName string) PackageData {
	return manifest.Repositories[packageName]
}

func (manifest *RepoManifest) SetRepository(packageName string, gitRemote string, gitBranch string, gitHash string, contentHash string) {
	manifest.Repositories[packageName] = PackageData{GitRemote: gitRemote, TrackingBranch: gitBranch, CurrentGitHash: gitHash, ContentHash: contentHash}
}

func (manifest *RepoManifest) HasRepository(packageName string) bool {
	_, ok := manifest.Repositories[packageName]
	return ok
}

func (manifest *RepoManifest) LikelyMatch(packageName string) string {
	for pkg := range manifest.Repositories {
		prefix := pkg
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
		if strings.HasPrefix(packageName, prefix) {
			return pkg
		}
	}
	return ""
}

func (manifest *RepoManifest) RemoveRepository(packageName string) {
	delete(manifest.Repositories, packageName)
}

// Iterate over the manifest in correct package name order.
func (manifest *RepoManifest) Each(f func(string, PackageData) error) error {
	pkgNames := []string{}
	for k := range manifest.Repositories {
		pkgNames = append(pkgNames, k)
	}
	sort.Strings(pkgNames)
	for _, pkgName := range pkgNames {
		if err := f(pkgName, manifest.Repository(pkgName)); err != nil {
			return err
		}
	}
	return nil
}

func (manifest *RepoManifest) packageLocation(pkgName string) string {
	return filepath.Join(manifest.srcDir, pkgName)
}

// Handler for the package reconcile functionality
func (manifest *RepoManifest) PkgReconcile(pkgName string) error {
	pkg, ok := manifest.Repositories[pkgName]
	if !ok {
		panic(fmt.Sprintf("package %s not found in manifest, should not get here", pkgName))
	}

	// Check the status of the package to be upgraded
	status, modified, err := manifest.packageStatus(pkgName)
	if err != nil {
		return err
	}

	// If we don't have any local mods, warn the user.
	if !modified {
		fmt.Println("WARNING: The checked in package has not been modified.")
		if status == statusUpToDate {
			fmt.Println("Also, that package is already up-to-date.")
		}
		fmt.Println("Exporting a repository anyway just so you can poke at it, but there's nothing to reconcile.")
	}

	fmt.Printf("Prepping package %s for reconcile\n", pkgName)

	// If the target package has local mods, abort.
	pathInBaseRepo := manifest.packageLocation(pkgName)
	if isModified, err := BaseGit().IsLocallyModified(pathInBaseRepo); err != nil {
		return err
	} else if isModified {
		return errors.Errorf("package %q has uncommitted changes! You can't reconcile while there are uncommited changes", pkgName)
	}

	// Path for this specific package to be exported to
	exportPath := filepath.Join(exportDir, pkgName)

	// Check if the directory we are trying to export to already exists, which would mean that we are in the middle of a reconcile
	if Exists(exportPath) {
		fmt.Println("It looks like you are already in the middle of a reconcile for this package")
		fmt.Println("To see your progress, check out the directory: " + exportPath)
		fmt.Println("To cancel the reconcile of this package, try: gorepoman reconcile " + pkgName + " cancel")
		return nil
	}

	// clone into the target dir
	if err := os.MkdirAll(exportPath, directoryPerm); err != nil {
		return err
	}

	// Clone, but don't check out, since we'll copy in the content.
	exportGit := TempGit(exportPath)
	params := []string{"clone", "--no-checkout"}
	if pkg.TrackingBranch != "" {
		params = append(params, "--branch", pkg.TrackingBranch)
	}
	params = append(params, pkg.GitRemote, ".")
	if _, err := exportGit.Command(params...); err != nil {
		return ErrWithMessagef(err, "failed to clone pkg %q from git remote %q", pkgName, pkg.GitRemote)
	}

	// Reset in the exported directory, reset master to our base revision.
	if _, err := exportGit.Command("reset", "--mixed", pkg.CurrentGitHash); err != nil {
		return err
	}

	// Copy our checked in source over to the exported directory
	if err := copyDir(pathInBaseRepo, filepath.Dir(exportPath)); err != nil {
		return err
	}

	isLocallyModified, err := exportGit.IsLocallyModified("")
	if err != nil {
		return err
	}

	if modified != isLocallyModified {
		fmt.Printf("That's weird, modified=%t but isLocallyModified=%t, is this a bug?\n", modified, isLocallyModified)
	}

	if isLocallyModified {
		// Create a commit containing exactly our local changes.
		if _, err := exportGit.Command("add", "-f", "."); err != nil {
			return err
		}

		if _, err := exportGit.Command("commit", "-m", "YOUR LOCAL CHANGES, CREATED BY REPOMAN"); err != nil {
			return err
		}
	}

	baseContentHash, err := BaseGit().TreeHash("HEAD", pathInBaseRepo)
	if err != nil {
		return err
	}

	contentHash, err := exportGit.TreeHash("HEAD", "")
	if err != nil {
		return err
	}

	if contentHash != baseContentHash {
		fmt.Printf("WARNING: reconcile repo at %q has contentHash %q; expected %q\n", exportPath, contentHash, baseContentHash)
	}

	fmt.Println("")
	fmt.Printf("Successfully exported package %s to working directory:\n%s\n", pkgName, exportPath)
	fmt.Println("Poke at it, shave the yaks, and then run this tool again with 'done' as a trailing param to move the changes back to our repo and strip the git metadata")
	fmt.Println("If you for some reason want to cancel, run the tool again with 'cancel' as a trailing param")

	return nil
}

// Handler for the completion of reconcile mode
func (manifest *RepoManifest) PkgReconcileDone(pkgName string) error {
	pkg, ok := manifest.Repositories[pkgName]
	if !ok {
		panic(fmt.Sprintf("package %s not found in manifest, should not get here", pkgName))
	}

	exportPath := filepath.Join(exportDir, pkgName)

	// Make sure a reconcile has been started
	if !Exists(filepath.Join(exportPath, ".git")) {
		return errors.Errorf("No export directory found at %q, no reconcile was ever started for package %s", exportDir, pkgName)
	}

	exportGit := TempGit(exportPath)
	// Make sure the export directory is clean.
	if modified, err := exportGit.IsLocallyModified(""); err != nil {
		return err
	} else if modified {
		return errors.Errorf("export directory %q has local modifications; cannot reconcile; please commit or revert your changes", exportPath)
	}

	// Grab the hash of the reconciled package
	exportHash, err := exportGit.CommitHash()
	if err != nil {
		return err
	}

	// Fetch latest version of the package so we have an up to date ancestry tree
	fmt.Println("Confirming ancestry between reconcile repo and upstream source")
	remote, err := exportGit.Remote() // Possibly update the remote.
	if err != nil {
		return err
	}
	remoteBranch, err := exportGit.RemoteTrackingBranch() // Possibly update branch
	if err != nil {
		return err
	}
	if remoteBranch == "" {
		// assume unchanged tracking branch
		remoteBranch = pkg.TrackingBranch
	}

	lsBranch := remoteBranch
	if lsBranch == "" {
		lsBranch = pkg.TrackingRef()
	}
	remoteVersion, err := exportGit.LsRemote(remote, lsBranch)
	if err != nil {
		return err
	}

	// Fetch the remote version into the temp repo so we can compute a merge base.
	if _, err := exportGit.Command("fetch", remote, remoteVersion); err != nil {
		return err
	}

	// now run merge base to determine the common ancestor between the reconciled repo and the remote.
	mergeBase, err := exportGit.MergeBase(remoteVersion, exportHash)
	if err != nil {
		return ErrWithMessagef(err, "could not find common ancestor between local commit %s and remote head %s; please fix", exportHash, remoteVersion)
	}

	// The "clean" content hash for change detection is the content hash of mergeBase
	mergeBaseContentHash, err := exportGit.TreeHash(mergeBase, "")
	if err != nil {
		return err
	}
	fmt.Printf("Found common ancestor commit %s with content hash %s\n", mergeBase, mergeBaseContentHash)
	fmt.Println("Copying reconcile directory back into your main repo")

	pathInBaseRepo := manifest.packageLocation(pkgName)
	if err := os.RemoveAll(pathInBaseRepo); err != nil {
		return ErrWithMessagef(err, "Could not remove %q from main repo to finish reconcile", pathInBaseRepo)
	}

	// Copy the contents of our exported directory back into our main repo
	if err := copyDir(exportPath, filepath.Dir(pathInBaseRepo)); err != nil {
		return err
	}
	// Delete the .git subdir from our main repo.
	if err := os.RemoveAll(filepath.Join(pathInBaseRepo, ".git")); err != nil {
		return ErrWithMessagef(err, "Could not remove %q/.git from main repo to finish reconcile", pathInBaseRepo)
	}
	// git add the results so we're ready to commit
	if _, err := BaseGit().Command("add", "-f", "--", pathInBaseRepo); err != nil {
		return err
	}

	manifest.SetRepository(pkgName, remote, remoteBranch, mergeBase, mergeBaseContentHash)
	fmt.Println("Writing new package manifest")
	if err := manifest.writeManifest(); err != nil {
		return err
	}

	// That's all done, manifest is rewritten, time to nuke the reconcile directory
	if err := os.RemoveAll(exportPath); err != nil {
		return errors.Wrapf(err, "Failed to clean up (remove) the reconcile directory at %s", exportPath)
	}

	fmt.Println("Reconcile complete!")
	return nil
}

// Handler for canceling reconcile mode
func (manifest *RepoManifest) PkgReconcileCancel(pkgName string) error {
	// Path for this specific package to be exported to
	exportPath := filepath.Join(exportDir, pkgName)

	if !Exists(exportPath) {
		return errors.Errorf("Unable to find a reconcile directory at %s - nothing to cancel. Did you ever start a reconcile for this package?", exportPath)
	}

	fmt.Println("Cancelling reconcile, cleaning up reconcile directory at " + exportPath)

	if err := os.RemoveAll(exportPath); err != nil {
		return errors.Wrapf(err, "Failed to remove reconcile working directory at %s", exportPath)
	}

	return nil
}

// Updates the specific package passed to it. This will also install all NEW dependencies, but will not recursively update the old ones
func (manifest *RepoManifest) PkgUpdate(pkgName string) error {
	// Check the status of the package to be upgraded
	status, modified, err := manifest.packageStatus(pkgName)
	if err != nil {
		return err
	}

	// If it is up to date, don't update
	if status == statusUpToDate {
		fmt.Println("No update needed! Version checked in is already the newest version")
		return nil
	} else if status == statusNonAncestor { // If it is not a true ancestor, don't update
		fmt.Println("While the current version is not the same as the new version, it is also not an ancestor. You can manually update by deleting and re-fetching the package, or resolve the issue using reconcile")
		return nil
	}

	// We need to confirm that the version of the package checked into our repo hasn't been modified
	// so that we don't blow away any changes made to it while updating
	if modified {
		// If something has been changed, we can't update
		fmt.Println("The checked in package has been modified, updating would blow away those changes.")
		fmt.Println("Please use reconcile mode to either upstream the changes or manually upate and rebase")
		return nil
	}

	// All clear to update the package
	fmt.Println("Update needed")
	fmt.Println("--Cleaning up old version--")
	if err := manifest.PkgDelete(pkgName, false); err != nil {
		return err
	}
	fmt.Println("")
	fmt.Println("--Installing new version--")
	// "go get" the requested package into the temp directory
	if err := goGet(pkgName, manifest.tmpDir); err != nil {
		return ErrWithMessagef(err, "Failed to 'go get' new package %s to be installed", pkgName)
	}
	if err := manifest.installPackages(); err != nil {
		return errors.Wrap(err, "Failed to install new packages")
	}
	// Write changes to the manifest if need be
	if manifest.Stale {
		fmt.Println("Writing new package manifest")
		if err := manifest.writeManifest(); err != nil {
			return err
		}
	} else {
		fmt.Println("No work to do!")
	}

	fmt.Printf("\nPackage %s successfully updated!\n", pkgName)
	return nil
}

type result struct {
	pkgName string
	output  string
	err     error
}

type byPackageName []result

func (p byPackageName) Len() int           { return len(p) }
func (p byPackageName) Less(i, j int) bool { return p[i].pkgName < p[j].pkgName }
func (p byPackageName) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (manifest *RepoManifest) EachConcurrent(f func(pkgName string, pkg PackageData) (string, error)) []result {
	resultCh := make(chan result, len(manifest.Repositories))
	for k, v := range manifest.Repositories {
		go func(pkgName string, pkg PackageData) {
			defer func() {
				if r := recover(); r != nil {
					err := errors.Errorf("panic: %+v", r)
					resultCh <- result{pkgName: pkgName, err: err}
				}
			}()
			output, err := f(pkgName, pkg)
			resultCh <- result{pkgName: pkgName, output: output, err: err}
		}(k, v)
	}

	var results []result
	for range manifest.Repositories {
		results = append(results, <-resultCh)
	}

	sort.Sort(byPackageName(results))
	return results
}

// List all of the packages that could use freshening
func (manifest *RepoManifest) PkgListStale() error {
	fmt.Println("Packages are stale if a newer version exists upstream.")
	fmt.Println("They should be brought up to date using 'gorepoman update <pkg>'.")

	git := BaseGit()
	results := manifest.EachConcurrent(func(pkgName string, pkg PackageData) (string, error) {
		remoteVersion, err := git.LsRemote(pkg.GitRemote, pkg.TrackingRef())
		if err != nil {
			return "", err
		}
		if remoteVersion != pkg.CurrentGitHash {
			return fmt.Sprintf("stale, local %s != remote %s", pkg.CurrentGitHash, remoteVersion), nil
		}
		return "", nil
	})

	for _, result := range results {
		if result.err != nil {
			return ErrWithMessagef(result.err, "error processing %s", result.pkgName)
		}
		if result.output == "" {
			continue
		}
		fmt.Printf("%s: %s\n", result.pkgName, result.output)
	}
	return nil
}

// List all of the packages that have had local changes made to them
func (manifest *RepoManifest) PkgListChanged() error {
	fmt.Println("Modified packages have been changed locally and don't match their corresponding")
	fmt.Println("upstream git hash.")

	git := BaseGit()
	results := manifest.EachConcurrent(func(pkgName string, pkg PackageData) (string, error) {
		pathInBaseRepo := manifest.packageLocation(pkgName)
		isLocallyModified, err := git.IsLocallyModified(pathInBaseRepo)
		if err != nil {
			return "", err
		}

		if isLocallyModified {
			return "working tree is dirty", nil
		}

		contentHash, err := git.TreeHash("HEAD", pathInBaseRepo)
		if err != nil {
			return "", err
		}

		if contentHash != pkg.ContentHash {
			return fmt.Sprintf("content modified, expected %s != actual %s", pkg.ContentHash, contentHash), nil
		}

		return "", nil
	})

	for _, result := range results {
		if result.err != nil {
			return ErrWithMessagef(result.err, "error processing %s", result.pkgName)
		}
		if result.output == "" {
			continue
		}
		fmt.Printf("%s: %s\n", result.pkgName, result.output)
	}
	return nil
}

const (
	statusUpToDate    = "up-to-date"
	statusStale       = "stale"
	statusNonAncestor = "non-ancestor"
)

// Compares local copy of the package to the remote version to determine its status
func (manifest *RepoManifest) packageStatus(pkgName string) (string, bool, error) {
	pkg, ok := manifest.Repositories[pkgName]
	if !ok {
		panic(fmt.Sprintf("package %s not found in manifest, should not get here", pkgName))
	}

	pathInBaseRepo := manifest.packageLocation(pkgName)

	// If the target package has local mods, abort.
	if isModified, err := BaseGit().IsLocallyModified(pathInBaseRepo); err != nil {
		return "", false, err
	} else if isModified {
		return "", false, errors.Errorf("package %q has uncommitted changes! You can't update while there are uncommited changes", pkgName)
	}

	// Are there local, committed changes?
	var isChanged bool
	if contentHash, err := BaseGit().TreeHash("HEAD", pathInBaseRepo); err != nil {
		return "", false, err
	} else {
		isChanged = contentHash != pkg.ContentHash
	}

	// Now check for up-to-date ness.
	remoteVersion, err := BaseGit().LsRemote(pkg.GitRemote, pkg.TrackingRef())
	if err != nil {
		return "", false, err
	}

	if remoteVersion == pkg.CurrentGitHash {
		return statusUpToDate, isChanged, nil
	}

	// We're either stale, or our remote ancestor is unrelated to the current master.
	// The only way to know is to do a bare clone and check ancestry.
	tmpRepo := filepath.Join(manifest.tmpDir, "src", pkgName+".git")
	if err := os.MkdirAll(tmpRepo, directoryPerm); err != nil {
		return "", false, err
	}
	defer os.RemoveAll(tmpRepo)

	tmpGit := TempGit(tmpRepo)
	if _, err := tmpGit.Command("init", "--bare"); err != nil {
		return "", false, ErrWithMessagef(err, "could not initialize temp repo for package %q at %q", pkgName, tmpRepo)
	}
	if _, err := tmpGit.Command("fetch", pkg.GitRemote, pkg.TrackingRef()); err != nil {
		return "", false, err
	}

	ancestor, err := tmpGit.MergeBase(remoteVersion, pkg.CurrentGitHash)
	if err != nil {
		return "", false, err
	}

	if ancestor == pkg.CurrentGitHash {
		// Our version is older than the remote
		return statusStale, isChanged, nil
	}

	// Our version is not a direct ancestor of the remote head. :(
	return statusNonAncestor, isChanged, nil
}

// Handles the delete command
// Will remove the package from our repo, then delete its hash data from the manifest
// Returns the manifest data after it has been mutated because of the delete
func (manifest *RepoManifest) PkgDelete(pkg string, writeManifest bool) error {
	// It is in the manifest, so the package should exist in the repo - try and delete it
	if err := os.RemoveAll(manifest.packageLocation(pkg)); err != nil {
		return errors.Wrapf(err, "Failed to delete package %s from the repository", pkg)
	}

	// Remove the deleted package from the map of packages
	fmt.Println("Deleting package " + pkg)
	manifest.RemoveRepository(pkg)
	if writeManifest {
		// Rewrite it using the updated package list
		fmt.Println("Updating Manifest")
		if err := manifest.writeManifest(); err != nil {
			return err
		}
	}

	return nil
}

// Handles the fetch command
func (manifest *RepoManifest) PkgFetch(pkg string) error {
	// "go get" the requested package into the temp directory
	if err := goGet(pkg, manifest.tmpDir); err != nil {
		return ErrWithMessagef(err, "Failed to 'go get' new package %s to be installed", pkg)
	}

	fmt.Println("Installing package " + pkg + " and possibly dependencies")
	fmt.Println("")

	// Actually install the new packages to our repo
	if err := manifest.installPackages(); err != nil {
		return errors.Wrap(err, "Failed to install new packages")
	}
	if manifest.Stale {
		fmt.Println("Writing new package manifest")
		if err := manifest.writeManifest(); err != nil {
			return err
		}
	} else {
		fmt.Println("No work to do!")
	}

	return nil
}

// Does the actual heavy lifting of installing of the new packages populated in the temp folder
func (manifest *RepoManifest) installPackages() error {
	// Scan the temp folder for all git repositories
	newRepos, err := discoverGit(manifest.tmpDir)
	if err != nil {
		return err
	}

	// Loop over each of the new repositories that we "go got"
	for _, newRepo := range newRepos {
		// Grab the name and hash of the new package we are looking at
		name := fmtPkgName(newRepo, manifest.tmpDir)

		tmpGit := TempGit(newRepo)
		remote, err := tmpGit.Remote()
		if err != nil {
			return err
		}

		newHash, err := tmpGit.CommitHash()
		if err != nil {
			return err
		}
		contentHash, err := tmpGit.TreeHash("HEAD", "")
		if err != nil {
			return err
		}
		// If the hash already exists in our repository
		if manifest.HasRepository(name) {
			// Determine if the git hash matches what we have checked in, and alert
			sameness := ""
			if manifest.Repository(name).CurrentGitHash != newHash {
				sameness = "a different git hash, maybe try the update function"
			} else {
				sameness = "the same git hash. Already installed, skipping"
			}
			fmt.Printf("Package %s already exists in our repo with %s \n", name, sameness)

		} else { // If we don't have the package checked in already...

			fmt.Printf("Installing package %s ... \n", name)
			// Create parent directory(ies) for the package (does nothing if it exists already)
			if err := os.MkdirAll(manifest.packageLocation(filepath.Dir(name)), directoryPerm); err != nil {
				return errors.Errorf("Failed to create parent directories for package %s", name)
			}
			// Remove git metadata from temp package
			if err := os.RemoveAll(newRepo); err != nil {
				return errors.Errorf("Failed to remove .git metadata from cloned package %s", name)
			}
			// Copy into new location in src
			path := manifest.packageLocation(name)
			if err := os.Rename(filepath.Dir(newRepo), path); err != nil {
				return errors.Wrapf(err, "Failed to move package %s from temporary directory to source folder", name)
			}

			if _, err := BaseGit().Command("add", "-f", "--", path); err != nil {
				return err
			}

			// We have made it this far without an error, theoretically everything succeeded, time to add the git hash
			// to the map of packages
			manifest.SetRepository(name, remote, "", newHash, contentHash)
			// Mark manifest for update
			manifest.Stale = true
			fmt.Printf("Successfully installed package %s \n", name)
		}
	}

	// Purely for ease of reading
	fmt.Println("")

	return nil
}

// Helper function to display all of the packages currently in the manifest file
// You can supply the list of packages if it is already loaded, or just nil, and it will be loaded here
func (manifest *RepoManifest) PkgList() error {
	return manifest.Each(func(pkgName string, pkg PackageData) error {
		fmt.Println(pkgName)
		return nil
	})
}

// Runs "go get" to clone down a package into our temp directory
func goGet(pkg string, tmpDir string) error {
	// Run go get
	_, err := execBinary("go", []string{"GOPATH=" + tmpDir}, "get", pkg)
	if err != nil {
		// If we downloaded the package, but couldn't find any gocode in the root
		if strings.Contains(err.Error(), "no Go files in") || strings.Contains(err.Error(), "no buildable Go source files") || strings.Contains(err.Error(), "build constraints exclude all Go files") || strings.Contains(err.Error(), "no non-test Go files in"){
			fmt.Printf("No go source found in package root %s - probably a single repo that contains sub packages\n", pkg)
			fmt.Println("The source was cloned successfully, should be fine")
			return nil
		}
		return errors.WithMessage(err, fmt.Sprintf("Failed to execute 'go get %s'", pkg))
	}
	return nil
}

func (manifest *RepoManifest) Path() string {
	return filepath.Join(manifest.srcDir, manifestFile)
}

// Read in the Manifest file containing the packages we already have fetched.
// Returns an error if there is no manifest file to be found.
func ReadManifest(srcDir, tmpDir string) (*RepoManifest, error) {
	manifest := RepoManifest{
		Repositories: make(RepoList),
		Stale:        false,
		srcDir:       srcDir,
		tmpDir:       tmpDir,
	}

	manifestPath := manifest.Path()
	if !Exists(manifestPath) {
		return nil, fmt.Errorf("No existing manifest found at %s\n", manifestPath)
	}

	bytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read manifest file")
	}
	// We wrote the JSON, so if it doesn't unmarshal it has probably been tampered with
	if err := json.Unmarshal(bytes, &manifest.Repositories); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshall JSON from manifest; did the file get modified externally?")
	}

	return &manifest, nil
}

// Create a new, empty Manifest file. Returns an error if a manifest file already exists.
func CreateManifest(srcDir, tmpDir string) (*RepoManifest, error) {
	manifest := RepoManifest{
		Repositories: make(RepoList),
		Stale:        false,
		srcDir:       srcDir,
		tmpDir:       tmpDir,
	}

	manifestPath := manifest.Path()
	if Exists(manifestPath) {
		return nil, fmt.Errorf("A manifest file already exists at %s", manifestPath)
	}
	if err := manifest.writeManifest(); err != nil {
		return nil, ErrWithMessagef(err, "Could not create new manifest at %s", manifestPath)
	}

	return &manifest, nil
}

// Writes a new manifest file given a map of packages and hashes
func (manifest *RepoManifest) writeManifest() error {
	pkgJson, err := json.MarshalIndent(manifest.Repositories, "", " ")
	if err != nil {
		return errors.Wrapf(err, "Failed to JSON-ify internal package list - %s", failureWarning)
	}
	manifestPath := manifest.Path()
	if err = ioutil.WriteFile(manifestPath, pkgJson, filePerm); err != nil {
		return errors.Wrapf(err, "Failed to write package JSON to new mainifest file - %s", failureWarning)
	}

	if _, err := BaseGit().Command("add", "-f", "--", manifestPath); err != nil {
		return err
	}
	return nil
}

// Walks through the temp directory and finds all git repositories present
// Currently we are only focusing on Git repos, eventually this could be expanded to work with svn and hg
func discoverGit(tmpDir string) ([]string, error) {
	// Structure the find command to find all .git folders in the temp directory
	cmdOut, err := execBinary("find", nil, filepath.Join(tmpDir, "src"), "-type", "d", "-name", ".git")
	if err != nil {
		return nil, errors.WithMessage(err, "Error executing find command to locate git repos")
	}

	if len(cmdOut) == 0 {
		// very unlikely
		return nil, errors.New("No git repositories found! The requested package could potentially be stored in svn or mercurial (unsupported)")
	}
	//Strip the trailing newline and split into seperate entries
	repos := strings.Split(strings.TrimSpace(cmdOut), "\n")

	return repos, nil
}

func fmtPkgName(pkgPath string, tmpDir string) string {
	prefix := tmpDir + "/src/"
	// This will need to be replaced with some logic if we want to support non git repos
	suffix := "/.git"
	// Strip off the prefix and the suffix from the path to the git folder
	return strings.TrimSuffix(strings.TrimPrefix(pkgPath, prefix), suffix)
}
