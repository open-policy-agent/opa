---
title: Contributing Code
---

We are thrilled that you're interested in contributing to OPA! This document
outlines some of the important guidelines when getting started as a new
contributor.

When contributing please consider the following pointers:

- **Testing:** Almost all code changes should be accompanied with tests. All CI
  checks must pass and there must be no warnings from the `make check` target.
- **Commits:** All code must be yours to contribute and commits must be signed off (see
  [Commit Messages](#commit-messages) below).
  Related commits must be squashed before they are merged (this can be done in
  the PR UI on GitHub).
- **Public APIs**: When you implement new features in OPA, consider whether the
  types/functions you are adding need to be exported. Prefer
  unexported types and functions as much as possible.

  If you need to share logic across multiple OPA packages, consider
  implementing it inside of the
  `github.com/open-policy-agent/opa/internal` package. The `internal`
  package is not visible outside of OPA.
- **Dependencies:**
  Avoid adding third-party dependencies (vendoring). OPA is designed to be minimal,
  lightweight, and easily embedded. Vendoring may make features _easier_ to
  implement however they come with their own cost for both OPA developers and
  OPA users (e.g., vendoring conflicts, security, debugging, etc.)
- **AI Tooling**: You can use generative AI tooling to assist your work on OPA,
  but please review our project's [AI Guidelines](#ai-guidelines) below before doing so to
  help us help you.

:::tip
Looking for developer environment set-up? Head over to the
['Contributing: Development'](./contrib-development/) page.
:::

### Commit Messages

Commit messages should explain _why_ the changes were made and should probably
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

```
This is my commit message

Signed-off-by: Random J Developer <random@developer.example.org>
```

Git has a `-s` command line option to do this automatically.

```sh
git commit -s -m 'This is my commit message'
```

Please review the [text of the DCO](https://developercertificate.org).

:::info
**Note:** If using AI or machine learning tools to assist in the authoring
of OPA patches, you must ensure the code you produce is compliant with the
DCO requirements, and OPA's license. All commits in your patch _must_ be signed
off by a human author.

The OPA maintainers reserve the right to request additional information about
patches and reject PRs where code origin cannot be verified.

See more in our [AI Guidelines](#ai-guidelines).
:::

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
Request. If you aren't sure, send a message to the OPA slack or post a comment
on the Pull Request.

### Vulnerability scanning

On each Pull Request, a series of tests will be run to ensure that the code
is up to standard. Part of this process is also to run vulnerability scanning
on the code and on the generated container image.

[Trivy](http://trivy.dev/) is used to run the aforementioned
vulnerability scanning. To install, follow the [installation instructions](https://trivy.dev/docs/latest/getting-started/).

To run the vulnerability scanning, on the code-base, run the following command:

```bash
trivy fs .
```

To run the vulnerability scanning on the container image, run the following command:

```bash
trivy image <Image tag>
```

If the tool catches any false positives, it's recommended to appropriately document them
in the `.trivyignore` file.

## AI Guidelines

We are really excited for you to contribute to OPA! In order for us (the OPA
maintainers) to help you effectively, we have some guidelines that we request
you follow:

1. Follow the
   [Linux Foundation Guidelines](https://www.linuxfoundation.org/legal/generative-ai)
   which require contributors to check (in summary):

   - AI tool restrictions: Contributors must ensure the AI tool's terms
     don't impose contractual limitations that conflict with the project's
     license.
   - Third-party content permissions: If generated output contains copyrighted
     materials from others, contributors must confirm proper permissions exist
     (via compatible license) and provide license information when contributing.

2. Respect maintainer time by:

   - Opening issues with clear proposals before starting work not already
     outlined in an existing issue.
   - Starting with small pull requests related to single issues (one at a time for new contributors).
   - Never using LLM output to respond to maintainer comments in PRs or issues.
     Reviewers are interested in knowing **your** reasoning about the code you
     submitted for review. Even if an LLM helped you write that code, it's yours
     to own and explain. Engaging an AI in discussions with human community
     members who took the time to review your code and provide you with personal
     feedback is considered disrespectful and will have your PR rejected.

3. Don't be afraid to get it wrong! We are friendly, and will answer questions
   you might have about contributing to our project. You can always:

   - Ask for clarification of a review comment if you don't understand it.
   - Ask for input on a technical implementation, ideally before investing your
     time into it.
   - Correct maintainers when you think they've misunderstood something.

   Together we can learn and build a better OPA!

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
