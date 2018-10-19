# gorepoman

`gorepoman` helps you easily manage and version control go dependencies.

Do you use open-source golang libraries? Do you struggle to track and manage versions, update stale
dependencies, and make custom edits as needed? Do you find the existing tool options somewhat lacking (git submodule, go get,
various go vendoring tools)?

`gorepoman` may be just what you need!  `gorepoman` is a super-powered wrapper around `go get` that maintains a
version-controlled JSON manifest.  The manifest contains the git SHAs associated with every managed package.

## Quick start demo

```bash
mkdir demo
cd demo
git init
export GOPATH=`pwd`

# bootstrap gorepoman with go get
go get github.com/fullstorydev/gorepoman/...
ls ./bin/gorepoman

# now that we're bootstrapped, nuke the source we fetched with `go get`, and re-fetch gorepoman using gorepoman!
rm -rf src/github.com/pkg/errors github.com/fullstorydev/gorepoman
./bin/gorepoman fetch github.com/fullstorydev/gorepoman

# gorepoman automatically git adds its work, commit the result
git status
git commit -m "gorepoman fetch github.com/fullstorydev/gorepoman"
```

## Subcommands

| subcommand                               | desc                                                         |
| ---------------------------------------- | ------------------------------------------------------------ |
| `gorepoman fetch <dep>`                    | Fetch the specified dep                                      |
| `gorepoman update <dep>`                   | Update the specified dep                                     |
| `gorepoman delete <dep>`                   | Delete the specified dep                                     |
| `gorepoman list`                           | List all managed deps                                        |
| `gorepoman list stale`                     | List all out-of-date deps                                    |
| `gorepoman list changed`                   | List all deps with local changes                             |
| `gorepoman reconcile <dep> [cancel\|done]` | (advanced) Reconcile a locally-changed dep with the upstream |

`<dep>` is e.g. 'github.com/pkg/errors'

## Features

- Quickly fetch new depedencies you need and version control them.

- Quickly list stale dependencies, and easily update.

- Transition gradually! `gorepoman` manages only the deps it has added.  You don't need to go through and
re-fetch all your go deps on day 1.

- If you've made local changes to one of your deps, advanced `reconcile` mode exports your dep into a real
(external) git repo, so you can easily merge in upstream changes or submit PRs / patches upstream.

- The generated manifest file is sorted and formatted for consistent and easy diff / merge.

## Caveats

- `gorepoman` is git-centric; not only must you use git yourself, but gorepoman can only manage third-party dependencies
that are also git. (This is almost never a problem in practice: everyone uses git!)
