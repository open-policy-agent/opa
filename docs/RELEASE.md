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
	git clone git@github.com:open-policy-agent/opa.git
	```

1. Edit CHANGELOG.md to update the Unreleased header (e.g., s/Unreleased/0.12.8/) and add any missing items to prepare for release.

1. Edit Makefile to set VERSION variable to prepare for release (e.g., s/VERSION := “0.12.8-dev”/VERSION := "0.12.8”/).

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

1. Edit Makefile to set VERSION variable to prepare for development (e.g., s/VERSION := “0.12.8”/VERSION = “0.12.9-dev”/).

1. Commit the changes and push to remote repository.

	```
	git commit -a -m “Prepare v<next_semvar> development”
	git push origin master
	```

## Building

1. Obtain copy of remote repository.

	```
	git clone git@github.com:open-policy-agent/opa.git
	```

1. Checkout release tag.

	```
	git checkout v<semver>
	```

1. Build binaries for target platforms.

	```
	make build GOOS=linux GOARCH=amd64
	make build GOOS=darwin GOARCH=amd64
	```

1. Build Docker image.

    ```
    make image
    ```

## Publishing

1. Open browser and go to https://github.com/open-policy-agent/opa/releases

1. Create a new release for the version.
	- Copy the changelog content into the message.
	- Upload the binaries.

1. Push the Docker image.

    ```
    make push
    ```

1. There may be documentation updates that should be released. See [site/README.md](../site/README.md) for steps to update the website.
