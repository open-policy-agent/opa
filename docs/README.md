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
[without Docker](#run-the-site-locally-without-docker).

### Generating Versioned Content ###
The site will show all versions of doc content from the tagged versions listed
in [RELEASES](./RELEASES).

To generate them run:
```shell
make load-docs
```

The content then will be placed into `docs/generated/docs/$VERSION/`.

### Run the site locally using Docker

If [Docker is running](https://docs.docker.com/get-started/):

```bash
make docker-serve
```

Open your browser to http://localhost:1313 to see the site running locally. The docs
are available at http://localhost:1313/docs.

### Run the site locally without Docker

To build and serve the site locally without using Docker, install the following packages
on your system:

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

#### Serving the site

From this directory:

```shell
make serve
```

Open your browser to http://localhost:1313 to see the site running locally. The docs
are available at http://localhost:1313/docs.


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
