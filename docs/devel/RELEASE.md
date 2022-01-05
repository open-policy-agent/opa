# Release Process

## Overview

The release process consists of two phases: versioning and publishing the release.

Versioning involves maintaining the following files:

- **CHANGELOG.md** - this file contains a list of all the important changes in each release.
- **Makefile** - the Makefile contains a VERSION variable that defines the version of the project.

The steps below explain how to update these files. In addition, the repository
should be tagged with the semantic version identifying the release.

Publishing involves creating a new *Release* on GitHub with the relevant
CHANGELOG.md snippet and uploading the binaries from the build phase.

## Versioning

The steps below assume an OPA development environment has configured for the
standard GitHub fork workflow. See [OPA Dev Instructions](DEVELOPMENT.md)

1. The following steps assume a remote named `upstream` exists that references the OPA source
   repository. As needed, add an `upstream` remote for the repository:

	```
	git remote add upstream git@github.com:open-policy-agent/opa.git
	git fetch --tags upstream
	```

1. Create a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)
   for GitHub with the 'read:org' scope. Export it to the `GITHUB_TOKEN` environment variable.

1. Execute the release-patch target to generate boilerplate patch. Give the semantic version of the release:

	```
	make release-patch VERSION=0.12.8 > ~/release.patch
	```

1. Apply the release patch to the working copy and preview the changes:

	```
	patch -p1 < ~/release.patch
	git diff
	```

	> Amend the changes as necessary, e.g., many of the Fixes and Miscellaneous
	> changes may not be user facing (so remove them). Also, if there have been
	> any significant API changes, call them out in their own sections.

1. Commit the changes and push to remote repository fork.

	```
	git add .
	git commit -s -m "Prepare v<version> release"
	git push origin main
	```

1. Create a Pull Request for the release preparation commit.

1. Once the Pull Request has merged fetch the latest changes and tag the commit to prepare for publishing:

    ```
    git fetch upstream
    git tag v<semver> upstream/main
    ```

    > Note: Ensure that tag is pointing to the correct commit ID! It must be the merged release preparation commit.

1. Execute the dev-patch target to generate boilerplate patch. Give the semantic version of the next release:

	```
	make dev-patch VERSION=0.12.9 > ~/dev.patch
	```

	> The semantic version of the next release typically increments the point version by one.

1. Apply the patch to the working copy and preview the changes:

	```
	patch -p1 < ~/dev.patch
	git diff
	```

1. Commit the changes and push to remote repository fork.

	```
	git commit -a -s -m "Prepare v<next_semvar> development"
	git push origin main
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

# Bugfix Release Process

The following steps assume a remote named `upstream` exists that references the OPA source
repository. As needed, add an `upstream` remote for the repository:

```
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

Update the `VERSION` variable in the Makefile and CHANGELOG, same workflow as a normal release.

```bash
make release-patch VERSION=0.14.1 > ~/release.patch
```

Apply the patch to the working copy and preview the changes:

```bash
patch -p1 < ~/dev.patch
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
the Pull Request into main or other release branches:

Once the Pull Request has merged fetch the latest changes and tag the commit to
prepare for publishing. Use the same instructions as defined above in normal
release [publishing](#publishing) guide (being careful to tag the appropriate commit).

Last step is to copy the CHANGELOG snippet and capabilities.json for the version to `main`. Create
a new PR with the version information added below the `Unreleased` section. Remove
any `Unreleased` notes if they were included in the bugfix release.
