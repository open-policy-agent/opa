# Release Process

## Overview

The release process consists of three phases: versioning, building, and
publishing.

Versioning involves maintaining the following files:

- **CHANGELOG.md** - this file contains a list of all the important changes in each release.
- **Makefile** - the Makefile contains a VERSION variable that defines the version of the project.
- **docs/website/RELEASES*** - this file determines which versions of documentation are displayed
  in the public [documentation](https://openpolicyagent.org/docs). __The first entry on the list is
  considered to be the latest.__

The steps below explain how to update these files. In addition, the repository
should be tagged with the semantic version identifying the release.

Building involves obtaining a copy of the repository, checking out the release
tag, and building the binaries.

Publishing involves creating a new *Release* on GitHub with the relevant
CHANGELOG.md snippet and uploading the binaries from the build phase.

## Versioning

1. Obtain a copy of repository.

	```
	git clone git@github.com:open-policy-agent/opa.git
	```

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

1. Commit the changes and push to remote repository.

	```
	git commit -a -s -m "Prepare v<version> release"
	git push origin master
	```

1. Tag repository with release version and push tags to remote repository.

	```
	git tag v<semver>
	git push origin --tags
	```

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

1. Commit the changes and push to remote repository.

	```
	git commit -a -s -m "Prepare v<next_semvar> development"
	git push origin master
	```

## Building

1. Obtain copy of remote repository.

	```
	git clone git@github.com:open-policy-agent/opa.git
	```

1. Execute the release target. The results can be found under _release/VERSION:

	```
	make release VERSION=0.12.8
	```

## Publishing

1. Open browser and go to https://github.com/open-policy-agent/opa/releases

1. Create a new release for the version.
	- Copy the changelog content into the message.
	- Upload the binaries.


## Notes

- The openpolicyagent/opa Docker image is automatically built and published to
  Docker Hub as part of the Travis-CI pipeline. There are no manual steps
  involved here.
- The docs and website should update and be published automatically. If they are not you can
  trigger one by a couple of methods:
	- Login to Netlify (requires permission for the project) and manually trigger a build.
	- Post to the build webhook via:
		```bash
		curl -X POST -d {} https://api.netlify.com/build_hooks/5cc3aa86495f22c7a368f1d2
		```

# Bugfix Release Process

If this is the first bugfix for the release, create the release branch from the
release tag:

```bash
git checkout -b release-0.14 v0.14.0
```

Otherwise, checkout the release branch and rebase on upstream:

```bash
git fetch upstream
git checkout release-0.14
git rebase upstream/release-0.14
```

Cherry pick the changes from master or other branches onto the bugfix branch:

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

> The generated CHANGELOG will likely need some manual adjustments for bug fix releases!

Commit this change:

```bash
git commit -s -a -m 'Prepare v0.14.1 release'
```

Push the release branch to your fork and open a Pull Request against the
upstream release branch. Be careful to open the Pull Request against the correct
upstream release branch. **DO NOT** open/merge the Pull Request into master or
other release branches:

```bash
git push origin release-0.14
```

Once the Pull Request has been merged you can tag the release at the commit
created above. Once the tag is pushed to `open-policy-agent/opa`, CI jobs will
automatically build and publish the Docker images and website updates.

Next build the release binaries and publish them to the [GitHub
releases](https://github.com/open-policy-agent/opa/releases) page along with
updating the CHANGELOG.md file on master.

```
make release VERSION=0.14.1
```

> The release binaries are located under `_release/<version>` in your working
> copy.

Last step is to copy the CHANGELOG snippet for the version to `master`. Create
a new PR with the version information added below the `Unreleased` section. Remove
any `Unreleased` notes if they were included in the bugfix release.
