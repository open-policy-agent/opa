# Contributing


## Report issues
If you find a bug, please report it, including environment and examples of current behavior and what you believe to be the correct behavior.  The clearer your description and information, the more likely it is someone will be able to make progress on it.

## Fix issues
Pull requests with fixes are totally welcome. Familiarize yourself with the folder structure and code style before you dive in.  Where possible fixes should include tests to prevent future regressions in functionality.  Also, if they apply and you have the ability, make fixes to both python and javascript implementations.

We use travis-ci.org to run build and test passes.  If you run `make` from the root folder locally, tests will run and should all pass before your pull request will be accepted.


# Folders
## js
## python
## web


# Branches
We use the `master` branch as the primrary development branch.

## Releases
Each platform has a branch that tracks to the latest release of that platform.

* `python-stable`
* `node-stable`
* `gh-pages`

## Functional Parity
Keeping the platforms in some semblance of functional parity is one of the key features of this project.  As such, there branches for the last time synchronization occured and when it stablized.

* `sync`
* `sync-stable`

## Attic
This project has been around for a while.  While some parts have improved significantly over time, others fell
into disrepair and were mothballed.

### PHP
There is a out-of-date version of the beautifier available on branch `attic-php`.  If you're interested
in using it feel free. If you plan to enhance it, please consider joining this project, and updating this
version to match current functionality.

### Other Languages
Versions of the beautifier adapted to other languages are at least two years out-of-date and are
available on branch `attic-other`.  Take a look and feel free to resurrect them, but know it's pretty
dusty back there.

### Generic Eval Unpacker
The `attic-genericeval` branch includes an unpacker that call `eval` on whatever source is passed to it.
Useful when working with source that unpacks itself when eval is called on it, but also unsafe.  We keep
it on this separate branch to keep it from hurting the other children.

# Publishing a Release
Each platform has it's own release process.

NOTE: Before you do any of these make sure the latest changes have passed the travis-ci build!

##Web
Merge changes from `master` to `gh-pages` branch.  This is very low cost and can be done whenever is convenient.

##Python
NOTE: For now, we'd like to keep python and node version numbers synchronized,
so if you publish a python release, you should publish a node release as well.

To perform these steps you will need:
1. A pypi user account from https://pypi.python.org/pypi?%3Aaction=register_form .
2. Permissions to the jsbeautifier package.  File an issue here on github and the appropriate person will help you.

We basically follow the simplest release path found at http://docs.python.org/2/distutils/packageindex.html . :

```bash
git clean -xfd
# replace 0.0.1 with the actual version number you want to use
NEW_VERSION=0.0.1
echo "__version__ = '$NEW_VERSION'" > python/jsbeautifier/__version__.py
git commit -am "Python $NEW_VERSION"
cd python
python setup.py register
python setup.py sdist bdist_wininst upload
git push
```

##Node
NOTE: For now, we'd like to keep python and node version numbers synchronized,
so if you plan to publish a node release, you should publish a python release *first*,
then perform the steps below.

To perform these steps you will need:
1. An npmjs.org user account from https://npmjs.org/signup .
2. Permissions to the js-beautify module on npmjs.org.  File an issue here on github and the appropriate person will help you.

Npm makes this process even simpler than python's and creates a tag for the release as well.

```bash
git clean -xfd
# replace 0.0.1 with the actual version number you want to use
NEW_VERSION=0.0.1
npm version $NEW_VERSION
npm publish .
git push --tags
```
