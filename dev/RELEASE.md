# Release Process

## Overview

The release process consists of three phases: versioning, building, and
publishing.

Versioning involves maintaining the CHANGELOG.md and version.go files inside
the repository and tagging the repository to identify specific releases.

Building involves obtaining a copy of the repository, checking out the release
tag, and building the packages.

Publishing involves creating a new *Release* on GitHub with the relevant
CHANGELOG.md snippet and uploading the packages from the build phase.

## Versioning

1. Obtain copy of remote repository.

	```
	git clone git@github.com/open-policy-agent/opa.git
	```

1. Edit CHANGELOG.md to update the Unreleased header (e.g., s/Unreleased/0.12.8/) and add any missing items to prepare for release.

1. Edit version/version.go to set Version variable to prepare for release (e.g., s/Version = “0.12.8-dev”/Version = "0.12.8”/).

1. Commit the changes and push to remote repository.

	```
	git commit -a -m “Prepare v<version> release”
	git push origin master
	```

1. Tag repository with release version and push tags to remote repository.

	```
	git tag v<semver>
	git push origin --tags
	```

1. Edit CHANGELOG.md to add back the Unreleased header to prepare for development.

1. Edit version/version.go to set Version variable to prepare for development (e.g., s/Version = “0.12.8”/Version = “0.12.9-dev”/).

1. Commit the changes and push to remote repository.

	```
	git commit -a -m “Prepare v<next_semvar> development”
	git push origin master
	```

## Building

1. Obtain copy of remote repository.

	```
	git clone git@github.com/open-policy-agent/opa.git
	```

1. Checkout release tag.

	```
	git checkout v<semver>
	```

1. Run command to build packages. This will produce a bunch of binaries (e.g., amd64/linux, i386/linux, amd64/darwin, etc.) that can be published (“distributions”).

	```
	make build CROSSCOMPILE="linux/amd64 darwin/amd64"
	```

## Publishing

1. Open browser and go to https://github.com/open-policy-agent/opa/releases

1. Create a new release for the version.
	- Copy the changelog content into the message.
	- Upload the distributions packages.

1. In addition to publishing the packages, there may be documentation updates that should be released. See [SITE.md](./SITE.md) for steps to update the website.
