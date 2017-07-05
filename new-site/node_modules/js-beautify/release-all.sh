#!/bin/bash

NEW_VERSION=$1

git checkout master

./generate-changelog.sh beautify-web/js-beautify || exit 1
git commit -am "Update Changelog for $NEW_VERSION"

# python
git clean -xfd || exit 1
echo "__version__ = '$NEW_VERSION'" > python/jsbeautifier/__version__.py
git commit -am "Python $NEW_VERSION"
cd python
python setup.py register
python setup.py sdist bdist_wininst upload
cd ..
git push

# node
git clean -xfd
npm version $NEW_VERSION
npm publish .
git push
git push --tags

# web
git clean -xfd
git checkout gh-pages && git fetch && git merge origin/master && git push || exit 1

git checkout master

