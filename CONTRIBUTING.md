# Contributing

Thanks for your interest in contributing to the Open Policy Agent (OPA) project!

# Where to start?

If you have questions, comments, or requests feel free to post on the mailing list or
create an issue on GitHub.

If you want to contribute code and you are new to the Go programming language, check out
the [DEVELOPMENT.md](./docs/devel/DEVELOPMENT.md) reference for help getting started.

We currently welcome contributions of all kinds. For example:

- Development of features, bug fixes, and other improvements.
- Documentation including reference material and examples.
- Bug and feature reports.

# Contribution process

Small bug fixes (or other small improvements) can be submitted directly via a Pull Request on GitHub.
You can expect at least one of the OPA maintainers to respond quickly.

Before submitting large changes, please open an issue on GitHub outlining:

- The use case that your changes are applicable to.
- Steps to reproduce the issue(s) if applicable.
- Detailed description of what your changes would entail.
- Alternative solutions or approaches if applicable.

Use your judgement about what constitutes a large change. If you aren't sure, send a message to the
mailing list or submit an issue on GitHub.

## Code Contributions

If you are contributing code, please consider the following:

- Most changes should be accompanied with tests.
- Commit messages should explain *why* the changes were made and should probably look like this:

        Description of the change in 50 characters or less

        More detail on what was changed. Provide some background on the issue
        and describe how the changes address the issue. Feel free to use multiple
        paragraphs but please keep each line under 72 characters or so.

- Related commits must be squashed before they are merged.
- All tests must pass and there must be no warnings from the `make check` target.
