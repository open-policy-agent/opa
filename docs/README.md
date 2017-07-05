# Docs

This directory contains all of the Markdown, HTML, CSS, and other assets needed
to build the [openpolicyagent.org](http://openpolicyagent.org) site. See the
section below for steps to build the site and test documentation changes
locally.

The raw OPA documentation can be found under the [book](./book) directory.

For development documentation see the [devel](./devel) directory.

## Site Updates

We use GitHub pages to host the website that includes all of the OPA
documentation. In order to update the website, you need to have write permission
on the open-policy-agent/opa repository.

### Prerequisites

If you want to build and serve the site locally, you need the following packages
installed on your system:

- npm
- [gulp](http://gulpjs.com/)
- [gitbook](https://github.com/GitbookIO/gitbook)

### Build and preview the docs locally

```
cd book
gitbook install
gitbook serve
```

> This will build the docs under `./book/_book`.

### Build and preview the entire site (front page and docs) locally

```
cd book; gitbook install; gitbook build; cd ..
npm install
gulp copy-book
gulp serve
```

### Build and preview (only) the front page locally

```
npm install
gulp
```

### Build site for release

From the root directory:

```
make docs
```

This will also serve the site on port 4000. The site will be saved to
`site.tar.gz` in the root directory.

### Update the website

Unzip the `site.tar.gz` file produced by building the site into the `gh-pages`
of this repository, add and commit the changes, and then push.
