# The OPA Website and Documentation
The content and tooling is separated into a few places:

[devel/](./devel/) - Developer documentation for OPA (not part of the website)


[website/](./website/) - This directory contains all of the Markdown, HTML, Sass/CSS,
and other assets needed to build the [openpolicyagent.org](https://openpolicyagent.org)
website. See the section below for steps to build the site and test documentation changes
locally. This content is not versioned for each release, it is common scaffolding for
the website.

[content/](./content/) - The raw OPA documentation can be found under the 
directory. This content is versioned for each release and should have all images
and code snippets alongside the markdown content files.

## Website Components

The website ([openpolicyagent.org](https://openpolicyagent.org)) and doc's hosted there
([openpolicyagent.org/docs](https://openpolicyagent.org/docs)) have a few components
involved with the buildings and hosting.

The static site is generated with [Hugo](https://gohugo.io/) which uses the markdown
in [content/](./content) as its content for pages under `/docs/*`. There is a script
to generate the previous supported versions, automated via `make generate`, and the
latest (current working tree) documentations is under `/docs/edge/*`.

The static site content is then hosted by [Netlify](https://www.netlify.com/). To
support backwards compatible URLs (from pre-netlify days) and to have the `latest`
version of the docs URLs work
[openpolicyagent.org/docs/latest](https://openpolicyagent.org/docs/latest) the site
relies on Netlify URL [redirects and rewrites](https://www.netlify.com/docs/redirects/)
which are defined in [website/layouts/index.redirects](./webiste/layouts/index.redirects)
and are build into a `_redirects` file when the Hugo build happens via
`make production-build` or `make preview-build`.

### How to Edit and Test

Because of the different components at play there are a few different ways to
test/view the website. The choice depends largely on what is being modified:

#### Full Site Preview

Go to [Netlify](https://www.netlify.com/) and log-in. Link to your public fork of
OPA on github and have it deploy a site. As long as it is public this is free
and can be configured to deploy test branches before opening PR's on the official
OPA github repo.

This approach gives the best simulation for what the website will behave like once
code has merged.


#### Modifying `content` (*.md)

The majorify of this can be done with any markdown renderer (typically built-in or
a plug-in for IDE's). The rendered output will be very similar to what Hugo will
generate.
 
> This excludes the Hugo shortcodes (places with `{{< SHORT_CODE >}}` in the markdown.
  To see the output of these you'll need to involve Hugo

#### Modifying the Hugo templates and/or website (HTML/CSS/JS)

The easiest way is to run Hugo locally in dev mode. Changes made will be reflected
immediately be the Hugo dev server. See 
[Run the site locally using Docker](#run-the-site-locally-using-docker)

> This approach will *not* include the Netlify redirects so urls like
 `http://localhost:1313/docs/latest/` will not work. You must navigate directly to
 the version of docs you want to test. Typically this will be
 [http://localhost:1313/docs/edge/](http://localhost:1313/docs/edge/).


#### Modifying the netlify config/redirects

This requires either using the [Full Site Preview](#full-site-preview) or using
the local dev tools as described below in:
[Run the site locally without Docker](#run-the-site-locally-without-docker)

The local dev tools will *not* give live updates as the content changes, but
will give the most accurate production simulation.

## Run the site locally

You can run the site locally [with Docker](#run-the-site-locally-using-docker) or
[without Docker](#run-the-site-locally-without-docker).

### Generating Versioned Content ###

> This *MUST* be done before you can serve the site locally!

The site will show all versions of doc content from the tagged versions listed
in [RELEASES](./RELEASES).

To generate them run:
```shell
make generate
```
The content then will be placed into `docs/website/generated/docs/$VERSION/`.

### Run the site locally using Docker

> Note: running with docker only uses the Hugo server and not Netlify locally.
This means that redirects and other Netlify features the site relies on will not work.

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
- The [Netlify dev CLI](https://www.netlify.com/products/dev/)

The site will be running from the Hugo dev server and fronted through netlify running
as a local reverse proxy. This more closely simulates the production environment but
gives live updates as code changes.

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

#### Serving the full site

From this directory:

```shell
make serve
```

Watch the console output for a localhost URL (the port is randomized). The docs are
available at http://localhost:$PORT/docs.


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
