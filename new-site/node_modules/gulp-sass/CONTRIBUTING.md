# Contributing to Gulp Sass

Gulp Sass is a very light-weight [Gulp](https://github.com/gulpjs/gulp) wrapper for [`node-sass`](https://github.com/sass/node-sass), which in turn is a Node binding for [`libsass`](https://github.com/sass/libsass), which in turn is a port of [`Sass`](https://github.com/sass/sass).

## Submitting Issues

* Before creating a new issue, perform a [cursory search](https://github.com/issues?utf8=%E2%9C%93&q=repo%3Adlmanning%2Fgulp-sass+repo%3Asass%2Fnode-sass+repo%3Asass%2Flibsass+repo%3Asass%2Fsass+repo%3Asass-eyeglass%2Feyeglass) in the Gulp Sass, Node Sass, Libsass, and main Sass repos to see if a similar issue has already been submitted. Please also refer to our [Common Issues and Their Fixes](https://github.com/dlmanning/gulp-sass/wiki/Common-Issues-and-Their-Fixes) page for some basic troubleshooting.
* You can create an issue [here](https://github.com/dlmanning/gulp-sass/issues). Please include as many details as possible in your report.
* Issue titles should be descriptive, explaining at the high level what it is about.
* Please include the version of `gulp-sass`, Node, and NPM you are using, as well as what operating system you are having a problem on.
* _Do not open a [pull request](#pull-requests) to resolve an issue without first receiving feedback from a `collaborator` or `owner` and having them agree on a solution forward_.
* Include screenshots and animated GIFs whenever possible; they are immensely helpful.
* Issues that have a number of sub-items that need to be complete should use [task lists](https://github.com/blog/1375%0A-task-lists-in-gfm-issues-pulls-comments) to track the sub-items in the main issue comment.


## Pull Requests

* **DO NOT ISSUE A PULL REQUEST WITHOUT FIRST [SUBMITTING AN ISSUE](#submitting-issues)**
* Pull requests should reference their related issues. If the pull request closes an issue, [please reference its closing in your commit messages](https://help.github.com/articles/closing-issues-via-commit-messages/). Pull requests not referencing any issues will be closed.
* Pull request titles should be descriptive, explaining at the high level what it is doing, and should be written in the same style as [Git commit messages](#git-commit-messages).
* Update the `CHANGELOG` with the changes made by your pull request, making sure to use the proper [Emoji](#emoji-cheatsheet).
* Follow our JavaScript styleguides. Tests will fail if you do not.
* Ensure that you have [EditorConfig](http://editorconfig.org/) installed in your editor of choice and that it is functioning properly.
* Do not squash or rebase your commits when submitting a Pull Request. It makes it much harder to follow your work and make incremental changes.
* Update the [CHANGELOG](#maintaining-thechangelog) with your changes.
* Branches should be made off of the most current `master` branch from `git@github.com:dlmanning/gulp-sass.git`
* Pull requests should be made into our [master](https://github.com/dlmanning/gulp-sass/tree/master) branch.

### Git Commit Messages

* Use the present tense (`"Add feature"` not `"Added Feature"`)
* Use the imperative mood (`"Move cursor to…"` not `"Moves cursor to…"`)
* Limit the first line to 72 characters or less
* Consider including relevant Emoji from our [Emoji cheatsheet](#emoji-cheatsheet)

## Creating a New Version

Versioning is done through [SEMVER](http://semver.org/). When creating a new version, create new release branch off of `master` with the version's name, and create a new tag with `v` prefixed with the version's name from that branch. 

For instance, if you are creating version `1.1.0`, you would create a branch `release/1.1.0` from `master` and create a tag `v1.1.0` from branch `release/1.1.0`.

### Maintaining the Changelog

The Changelog should have a list of changes made for each version. They should be organized so additions come first, changes come second, and deletions come third. Version numbers should be 2nd level headers with the `v` in front (like a tag) and the date of the version's most recent update should be underneath in italics.

Changelog messages do not need to cover each individual commit made, but rather should have individual summaries of the changes made. Changelog messages should be written in the same style as [Git commit messages](#git-commit-messages).

## Emoji Cheatsheet

When creating creating commits or updating the CHANGELOG, please **start** the commit message or update with one of the following applicable Emoji. Emoji should not be used at the start of issue or pull request titles.

* :art: `:art:` when improving the format/structure of the code
* :racehorse: `:racehorse:` when improving performance
* :memo: `:memo:` when writing long-form text (documentation, guidelines, principles, etc…)
* :bug: `:bug:` when fixing a bug
* :fire: `:fire:` when removing code or files
* :green_heart: `:green_heart:` when fixing the CI build
* :white_check_mark: `:white_check_mark:` when adding tests
* :lock: `:lock:` when dealing with security
* :arrow_up: `:arrow_up:` when upgrading dependencies
* :arrow_down: `:arrow_down:` when downgrading dependencies
* :shirt: `:shirt:` when removing linter warnings
* :shipit: `:shipit:` when creating a new release
