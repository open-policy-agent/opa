---
title: Contributing Documentation
---

Contributing to our documentation is one of the best ways to get started
contributing to the OPA project. The OPA documentation often the first place
people go for help and so any improvements can be very impactful.
**Thank you in advance for contributing to our documentation!**

## Local Development

Those contributing to the OPA website and documentation must first install the
Node packages to run it. The following commands are run relative to the
[`docs/` directory](https://github.com/open-policy-agent/opa/tree/main/docs).

```bash
cd docs
make install
```

Once these have been installed, a live updating development server can be
started by running:

```bash
make dev
```

Note, that the server will not restart when you change the
`docusaurus.config.js`.

If you are working on the website code (not only the documentation content)
you might find the following command useful to reload the site when changing the
configuration file too:

```bash
find . -name docusaurus.config.js -o -name sidebars.js -o -name sidebars.js | entr -c -r make dev
```

You can run a local build of the website using the `build`. This will also show
a summary of any broken links or anchors.

```bash
make build
```

This will build a version of the site as it is deployed to the Netlify CDN.

## `docs` Directory Structure

- `docs/` contains the main documentation content.
- `src/` is where custom pages and components are defined. In addition, some
  dynamically processed content such as that for the `/ecosystem/*` pages is
  managed here.
  - `pages` contains the designs and assets used for custom pages not under
    `/docs/`.
  - `lib` contains shared Javascript functions and sidebar configuration.
  - `theme` contains customizations to the Docusaurus theme components.
  - `data/cli.json` is automatically updated from the make task
    `generate-cli-docs`.
- `functions/` is used loaded by Netlify to run a number of edge functions for
  interactive purposes or complex redirects.

## Documentation File Format

Documentation is written primarily as Markdown. However, the use of React
components is available too. A number of components are used to provide more
advanced features such as:

- Interactive Rego examples
- Two column layouts
- Linking to relevant pages in the `/ecosystem` section
- Mermaid diagrams containing asset references

## Create a local branch

To get started, fork the OPA repository and create a local branch for your Docs changes.
Check out the [Development Guide](./contrib-development/#fork-clone-create-a-branch)
if you need some help setting this up.

## Update Existing Docs

Navigate to the
[docs](https://github.com/open-policy-agent/opa/blob/main/docs/docs)
folder in your local environment. Each top level item in the documentation nav
will have an associated markdown file in the documentation folder. Locate the
file you wish to update and confirm the title in the YAML frontmatter matches. Once
you've located the correct page, edit the markdown page as necessary.

## Adding New Pages

In the case where you want to add a topic that doesn't fit nicely into any of
the existing pages, it may make sense to add a new page. Create a markdown file
in the content folder and add the appropriate YAML frontmatter heading. Aside
from the title, you may wish to set `sidebar_position`.

You may also wish to update `src/lib/sidebars.js` to place your new page in the
correct location.

## Test your changes

Once you have made your updates, the next step is to test that they look as
expected. To test your changes, you can run `make dev`.

## Submit a Pull Request

Once you've tested your changes and you're happy with how they look, commit them
to your branch and open a pull request. If this is your first time opening a
pull request with the OPA repository, check out the
[Contributing Guide](./contributing).
Once your PR has been received a Netlify preview will be automatically created,
check the PR for a unique link.

## Having trouble

Reach out in the
[#contributors](https://openpolicyagent.slack.com/archives/C02L1TLPN59)
channel to ask for help.

## Sub-project Documentation

Documentation for the OPA sub-projects each have their own home. Check out their
documentation sites to see how to contribute.

- [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/)
  docs for Kubernetes Admission Control
- [Conftest](https://www.conftest.dev/)
  docs for validating your structured configuration data, like YAML and HCL files
