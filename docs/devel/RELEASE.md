# Release Process

## Overview

The release process consists of two phases: versioning and publishing the release.

Versioning involves maintaining the following files:

- **CHANGELOG.md** - this file contains a list of all the important changes in each release.
- **v1/version/version.go** - this file contains the `Version` variable that defines the
  version of the project. The Makefile's `VERSION` is read from it.
- **capabilities.json**, **capabilities/v\<version\>.json**, **builtin_metadata.json**
  and **v1/ast/version_index.json** - generated files that record what the release supports.

These are maintained by the `release-prepare` and `dev-prepare` make targets, which
wrap the tooling in [build/release](../../build/release). The steps below explain how
to run them. In addition, the repository should be tagged with the semantic version
identifying the release.

Publishing involves creating a new _Release_ on GitHub with the relevant
CHANGELOG.md snippet and uploading the binaries from the build phase.

> Note: This release process is subject to change without notice.

## Release Cadence

There are two version tracks for the OPA project:

1. Release Candidate (vX.Y.Z-rc.A)
2. Stable (vX.Y.Z)

A new version of OPA is scheduled to release on the last Friday of every month. At the beginning of that week,
a release candidate branch (`release-<major>.<minor>-rc.0`) will be created from the main branch and a release
candidate tag (`v<major>.<minor>.0-rc.0`) will be created based on the release candidate branch for pre-release. Once the pre-release
is published, users are encouraged to try out the features, bug fixes in the release candidate. If regressions or bugs
are detected, they need to get fixed before cutting the stable release. It is not recommended to use OPA release
candidates in a production environment. The stable release that comes out after the release candidate may be identical
to the release candidate if no other features or bug fixes are introduced to the main branch in between.

See the next section for details on cutting an individual release.

## Versioning

The steps below assume an OPA development environment has configured for the
standard GitHub fork workflow. See [OPA Dev Instructions](DEVELOPMENT.md)

1. The following steps assume a remote named `upstream` exists that references the OPA source
   repository. As needed, add an `upstream` remote for the repository:

   ```sh
   git remote add upstream git@github.com:open-policy-agent/opa.git
   git fetch --tags upstream
   ```

   Note: This stage can fail if you have not registered an [SSH key](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/adding-a-new-ssh-key-to-your-github-account)
   on your GitHub account.

1. Create a release branch off of `main`, to ensure you don't mangle your
   fork while creating the release:

   ```
   git checkout -b release-v<version> origin/main
   ```

1. Authenticate with GitHub, either by running `gh auth login` or by exporting a
   [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
   with the `public_repo` scope to the `GITHUB_TOKEN` environment variable.

1. Execute the release-prepare target. Give the semantic version of the release:

   ```
   make release-prepare VERSION=1.19.0
   ```

   This fills in the CHANGELOG section for the release and updates
   `v1/version/version.go`, `capabilities.json`, `capabilities/v<version>.json`,
   `builtin_metadata.json` and `v1/ast/version_index.json`. The changes are made
   directly in the working tree.

   > By default the release range starts at the latest tag reachable from `HEAD`.
   > Pass `LAST_VERSION=v<previous>` to override it.

1. Review the changes:

   ```
   git status
   git diff
   ```

   > Amend the CHANGELOG as necessary. Many entries are not user facing (so
   > remove them), and entries are worth regrouping into topical sections.
   > If there have been any significant API changes, call them out in their own sections.
   > The tool also prints a review checklist to stderr, which is worth reading before committing.

1. Commit the changes and push to remote repository fork.

   ```
   git add .
   git commit -s -m "Prepare v<version> release"
   git push origin release-v<version>
   ```

1. Create a Pull Request for the release preparation commit.

1. Once the Pull Request has merged fetch the latest changes and tag the commit to prepare for publishing:

   ```
   git fetch upstream
   git tag v<semver> upstream/main
   ```

   > Note: Ensure that tag is pointing to the correct commit ID! It must be the merged release preparation commit.

1. Create a new branch for the development preparation work:

   ```
   git checkout -b dev-v<next_semvar> origin/main
   ```

1. Execute the dev-prepare target. Give the semantic version of the next release:

   ```
   make dev-prepare VERSION=1.20.0
   ```

   > The semantic version of the next release typically increments the point version by one.

   This sets `Version` to `<next_semvar>-dev` and adds an `## Unreleased`
   heading to the CHANGELOG.

1. Review the changes:

   ```
   git diff
   ```

1. Commit the changes and push to remote repository fork.

   ```
   git commit -a -s -m "Prepare v<next_semvar> development"
   git push origin dev-v<next_semvar>
   ```

1. Create a Pull Request for the development preparation commit.

## Publishing

1. Push the release tag to remote source repository.

   ```
   git push upstream v<semver>
   ```

   > Note: Only OPA maintainers will have permissions to perform this step.

1. Open browser and go to [https://github.com/open-policy-agent/opa/releases](https://github.com/open-policy-agent/opa/releases)

1. Update the draft release (may take up to 20 min for the draft to become
   available, track its process under
   [https://github.com/open-policy-agent/opa/actions](https://github.com/open-policy-agent/opa/actions)).
   Ensure everything looks OK and publish when ready.

## Notes

- The openpolicyagent/opa Docker image is automatically built and published to
  Docker Hub as part of the Travis-CI pipeline. There are no manual steps
  involved here.
- The docs and website should update and be published automatically. If they are not you can
  trigger one by a couple of methods:
  - Login to Netlify (requires permission for the project) and manually trigger a build.
  - Post to the build webhook via:
    ```bash
    curl -X POST -d {} https://api.netlify.com/build_hooks/612e8941ffe30d2902bcce80
    ```
- The Algolia search index is automatically updated when the site is crawled daily at 20:30 (UTC). The
  crawling process takes around 25 minutes to complete and can be triggered from
  [crawler.algolia.com](https://crawler.algolia.com) (login details required).

## Bugfix Release Process

The following steps assume a remote named `upstream` exists that references the OPA source
repository. As needed, add an `upstream` remote for the repository:

```sh
git remote add upstream git@github.com:open-policy-agent/opa.git
git fetch --tags upstream
```

If this is the first bugfix for the release, create the release branch from the
release tag and push to the source repository.

```bash
git checkout -b release-0.14 v0.14.0
git push upstream release-0.14
```

Otherwise, checkout the release branch and sync with `upstream` (as needed):

```bash
git fetch upstream
git checkout release-0.14
git reset --hard upstream/release-0.14
```

Cherry pick the changes from main or other branches onto the bugfix branch:

```bash
git cherry-pick -x <commit-id>
```

> Using `-x` helps to keep track of where the commit came from originally

Update the version and CHANGELOG, same workflow as a normal release:

```bash
make release-prepare VERSION=0.14.1
```

Review the changes:

```bash
git status
git diff
```

> The generated CHANGELOG will likely need some manual adjustments for bugfix releases!

Commit this change and push to fork:

```bash
git commit -s -a -m 'Prepare v0.14.1 release'
git push origin release-0.14
```

Open a Pull Request against the upstream release branch. Be careful to open the
Pull Request against the correct upstream release branch. **DO NOT** open/merge
the Pull Request into main or other release branches.

> Note: Make sure to do a "Rebase and merge" and NOT a squash when merging the PR, to preserve the cherry-picked commits.
> Alternatively, the cherry-picks can be pushed to `upstream` before submitting the PR.

Once the Pull Request has merged fetch the latest changes and tag the commit to
prepare for publishing. Use the same instructions as defined above in normal
release [publishing](#publishing) guide (being careful to tag the appropriate commit).

Last step is to copy the CHANGELOG snippet and generated files
(builtin_metadata.json and capabilities.json) for the version to `main`. Create
a new PR with the version information added below the `Unreleased` section.
Remove any `Unreleased` notes if they were included in the bugfix release.
