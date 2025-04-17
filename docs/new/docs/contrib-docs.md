---
title: Contributing Docs
kind: contrib
weight: 2
---

Contributing to our docs is one of the best ways to get started contributing to the OPA project. The OPA docs are one of the first places most people go for help, so any changes are impactful for the community. Thanks in advance for contributing to the docs.

## Sub-project docs

Docs for the OPA sub-projects each have their own home. Check out their docs to see how to contribute.

* [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) docs for Kubernetes Admission Control
* [Conftest](https://www.conftest.dev/) docs for validating your structured configuration data, like YAML and HCL files 


## Docs File Structure 

- [devel/](https://github.com/open-policy-agent/opa/blob/main/docs/devel) - Developer documentation for OPA (not part of the website)
- [website/](https://github.com/open-policy-agent/opa/blob/main/docs/website) - This directory contains all of the Markdown, HTML, Sass/CSS, and other assets needed to build the [openpolicyagent.org](https://openpolicyagent.org) website. See the section below for steps to build the site and test documentation changes locally. This content is not versioned for each release, it is common scaffolding for the website.
- [content/](https://github.com/open-policy-agent/opa/blob/main/docs/content) - The raw OPA documentation can be found under the directory. This content is versioned for each release and should have all images and code snippets alongside the markdown content files.
- [content/integrations, content/organizations, content/softwares](https://github.com/open-policy-agent/opa/blob/main/docs/content) - the source for data used to generate the [OPA Ecosystem](https://www.openpolicyagent.org/docs/latest/ecosystem/)

## Markdown Page Structure

Each page starts with a Frontmatter block that provides the necessary metadata to format the Docs website. For example, the frontmatter for this page is as follows:

```YAML
---
title: Contributing Docs #Title of the page
kind: contrib #Section the page belongs to
weight: 2 #Order the page should appear in the left hand nav
---
```

After the Frontmatter block, each page uses standard markdown formatting. For a primer on Markdown, check out the [GitHub Markdown Guide](https://docs.github.com/en/github/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax). 

## Create a local branch

To get started, fork the OPA repository and create a local branch for your Docs changes. Check out the [Development Guide](../contrib-development/#fork-clone-create-a-branch) if you need some help setting this up. 

## Update Existing Docs

Navigate to the [content/](https://github.com/open-policy-agent/opa/blob/main/docs/content) folder in your local environment. Each top level item in the documentation nav will have an associated markdown file in the content folder. Locate the file you wish to update and confirm the title in the Frontmatter matches. Once you've located the correct page, edit the markdown page as necessary. 

## Adding New Pages

In the case where you want to add a topic that doesn't fit nicely into any of the existing pages, it may make sense to add a new page. Create a markdown file in the content folder and add the appropriate Frontmatter heading. Aside from the title, you will need to specify the `kind` (section in the docs) and `weight` (order the page appears). 

## Test your changes

Once you have made your updates, the next step is to test that they look as expected. To test your changes, generate a local preview with your updated files and preview them with Netlify.

Summary of steps:
1. Install dependencies: [Hugo](https://github.com/open-policy-agent/opa/tree/main/docs#installing-hugo), [NodeJS](https://nodejs.org), and [Netlify CLI](https://www.netlify.com/products/dev/)
1. Build artifacts: `make build`
1. Generate HTML files: `make docs-serve-local` (*This can take up to 5 min*)
1. Start preview server: `netlify dev`

For detailed instructions on setting up local preview, and remote previews with Netlify, check out the docs [README page](https://github.com/open-policy-agent/opa/blob/main/docs/README.md#how-to-edit-and-test).

## Submit a Pull Request

Once you've tested your changes and you're happy with how they look, commit them to your branch and open a pull request. If this is your first time opening a pull request with the OPA repository, check out the [Contributing Guide](../contributing). Once your PR has been received a Netlify preview will be automatically created, check the PR for a unique link.

## Having trouble

Reach out in the [#contributors](https://openpolicyagent.slack.com/archives/C02L1TLPN59) channel to ask for help. 