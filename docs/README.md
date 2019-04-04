# The OPA Website and Documentation

This directory contains all of the Markdown, HTML, Sass/CSS, and other assets needed
to build the [openpolicyagent.org](https://openpolicyagent.org) website. See the
section below for steps to build the site and test documentation changes
locally.

The raw OPA documentation can be found under the [content/docs](./content/docs)
directory.

> ### Developing OPA
> For documentation on developing OPA, see the [devel](./devel) directory.

## Run the site locally

You can run the site locally [with Docker](#run-the-site-locally-using-docker) or
[without Docker](#run-the-site-locally-without-docker). Regardless of your method,
you'll need install [npm](https://www.npmjs.com/get-npm).

### Run the site locally using Docker

To run the site locally using [Docker](https://docker.com), first install the
necessary static assets using npm:

```bash
npm install
```

Then, if [Docker is running](https://docs.docker.com/get-started/):

```bash
make docker-server
```

Open your browser to http://localhost:1313 to see the site running locally. The docs
are available at http://localhost:1313/docs.

### Run the site locally without Docker

To build and serve the site locally without using Docker, install the following packages
on your system:

- [npm](https://npmjs.org)
- The [Hugo](#installing-hugo) static site generator

#### Installing Hugo

Running the website locally requires installing the [Hugo](https://gohugo.io) static
site generator. The required version of Hugo is listed in the
[`netlify.toml`](./netlify.toml) configuration file (see the `HUGO_VERSION` variable).

Installation instructions for Hugo can be found in the [official
documentation](https://gohugo.io/getting-started/installing/).

Please note that you need to install the "extended" version of Hugo (with built-in
support) to run the site locally. If you get errors like this, it means that you're
using the non-extended version:

```
error: failed to transform resource: TOCSS: failed to transform "sass/style.sass" (text/x-sass): this feature is not available in your current Hugo version
```

#### Installing static assets

The OPA website requires some static assets installable via npm:

```bash
npm install
```

#### Serving the site

From this directory:

```shell
make serve
```

Open your browser to http://localhost:1313 to see the site running locally. The docs
are available at http://localhost:1313/docs.

> The local version of the site is different from the production site at
> https://openpolicyagent.org in important ways. When running the site locally,
> the documentation you'll see is from the current branch, not from any past release.
> Thus, there is no version selector or concept of documentation versions in the
> "dev" site. The documentation that you see when running the site locally will be
> visible on the production site only when that documentation has been included as
> part of an OPA release.

## Site updates

The OPA site is automatically published using [Netlify](https://netlify.com). Whenever
changes in this directory are pushed to `master`, the site will be re-built and
re-deployed.

## Checking links

To check the site's links, first install the [`htmlproofer`](https://github.com/gjtorikian/html-proofer) Ruby gem:

```bash
gem install htmlproofer
```

Then run:

```bash
make linkcheck
```
