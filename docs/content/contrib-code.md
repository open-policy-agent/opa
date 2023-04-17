---
title: Contributing Code
kind: contrib
weight: 3
---

## Code Contributions

If you are contributing code, please consider the following:

- Most changes should be accompanied with tests.
- All commits must be signed off (see next section).
- Related commits must be squashed before they are merged.
- All tests must pass and there must be no warnings from the `make
  check` target.

If you are new to Go, consider reading [Effective
Go](https://golang.org/doc/effective_go.html) and [Go Code Review
Comments](https://github.com/golang/go/wiki/CodeReviewComments) for
guidance on writing idiomatic Go code.

When you implement new features in OPA, consider whether the
types/functions you are adding need to be exported. Prefer
unexported types and functions as much as possible.

If you need to share logic across multiple OPA packages, consider
implementing it inside of the
`github.com/open-policy-agent/opa/internal` package. The `internal`
package is not visible outside of OPA.

Avoid adding third-party dependencies (vendoring). OPA is designed to be minimal,
lightweight, and easily embedded. Vendoring may make features _easier_ to
implement however they come with their own cost for both OPA developers and
OPA users (e.g., vendoring conflicts, security, debugging, etc.)

### Commit Messages

Commit messages should explain *why* the changes were made and should probably
look like this:

```
Description of the change in 50 characters or less

More detail on what was changed. Provide some background on the issue
and describe how the changes address the issue. Feel free to use multiple
paragraphs but please keep each line under 72 characters or so.
```

If your changes are related to an open issue (bug or feature), please include
the following line at the end of your commit message:

```
Fixes #<ISSUE_NUMBER>
```

If the changes are isolated to a specific OPA package or directory please
include a prefix on the first line of the commit message with the following
format:

```
<package or directory path>: <description>
```

For example, a change to the `ast` package:
```
ast: Fix X when Y happens

<Details...>

Fixes: #123
Signed-off-by: Random J Developer <random@developer.example.org>
```

or a change in the OPA website content (found in `./docs/content`):
```
docs/website: Add X to homepage for Y

<Details...>

Fixes: #456
Signed-off-by: Random J Developer <random@developer.example.org>
```

### Developer Certificate Of Origin

The OPA project requires that contributors sign off on changes submitted to OPA
repositories.
The [Developer Certificate of Origin (DCO)](https://developercertificate.org/)
is a simple way to certify that you wrote or have the right to submit the code
you are contributing to the project.

The DCO is a standard requirement for Linux Foundation and CNCF projects.

You sign-off by adding the following to your commit messages:

    This is my commit message

    Signed-off-by: Random J Developer <random@developer.example.org>

Git has a `-s` command line option to do this automatically.

    git commit -s -m 'This is my commit message'

You can find the full text of the DCO here: https://developercertificate.org/

### Code Review

Before a Pull Request is merged, it will undergo code review from other members
of the OPA community. In order to streamline the code review process, when
amending your Pull Request in response to a review, do not squash your changes
into relevant commits until it has been approved for merge. This allows the
reviewer to see what changes are new and removes the need to wade through code
that has not been modified to search for a small change.

When adding temporary patches in response to review comments, consider
formatting the message subject like one of the following:

- `Fixup into commit <commit ID> (squash before merge)`
- `Fixed changes requested by @username (squash before merge)`
- `Amended <description> (squash before merge)`

The purpose of these formats is to provide some context into the reason the
temporary commit exists, and to label it as needing squashed before a merge
is performed.

It is worth noting that not all changes need be squashed before a merge is
performed. Some changes made as a result of review stand well on their own,
independent of other commits in the series. Such changes should be made into
their own commit and added to the PR.

If your Pull Request is small though, it is acceptable to squash changes during
the review process. Use your judgement about what constitutes a small Pull
Request.  If you aren't sure, send a message to the OPA slack or post a comment
on the Pull Request.

### Vulnerability scanning

On each Pull Request, a series of tests will be run to ensure that the code
is up to standard. Part of this process is also to run vulnerability scanning
on the code and on the generated container image.

[Trivy](https://aquasecurity.github.io/trivy/) is used to run the aforementioned
vulnerability scanning. To install, follow the [installation instructions](
https://aquasecurity.github.io/trivy/v0.29.2/getting-started/installation/).

To run the vulnerability scanning, on the code-base, run the following command:

```bash
$ trivy fs .
```

To run the vulnerability scanning on the container image, run the following command:

```bash
$ trivy image <Image tag>
```

If the tool catches any false positives, it's recommended to appropriately document them
in the `.trivyignore` file.

## Contribution process

Small bug fixes (or other small improvements) can be submitted directly via a
[Pull Request](https://github.com/open-policy-agent/opa/pulls) on GitHub.
You can expect at least one of the OPA maintainers to respond quickly.

Before submitting large changes, please open an issue on GitHub outlining:

- The use case that your changes are applicable to.
- Steps to reproduce the issue(s) if applicable.
- Detailed description of what your changes would entail.
- Alternative solutions or approaches if applicable.

Use your judgement about what constitutes a large change. If you aren't sure,
send a message in 
[#contributors](https://openpolicyagent.slack.com/archives/C02L1TLPN59) on Slack 
or submit [an issue on GitHub](https://github.com/open-policy-agent/opa/issues).
