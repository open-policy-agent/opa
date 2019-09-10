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
  To see the output of these you'll need to involve Hugo. Additionally, to validate
  and view live code blocks, a full site build is required (e.g. `make serve`,
  details below). See the "Live Code Blocks" section for more information on
  how to write them.

#### Modifying the Hugo templates and/or website (HTML/CSS/JS)

The easiest way is to run Hugo locally in dev mode. Changes made will be reflected
immediately be the Hugo dev server. See 
[Run the site locally using Docker](#run-the-site-locally-using-docker)

> This approach will *not* include the Netlify redirects so urls like
  `http://localhost:1313/docs/latest/` will not work. You must navigate directly to
  the version of docs you want to test. Typically this will be
  [http://localhost:1313/docs/edge/](http://localhost:1313/docs/edge/).
  It will also not include the processing required for live code blocks
  to show up correctly.


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

This will attempt to fetch the latest tags from git. The fetch will be skipped
if the `OFFLINE` environment variable is defined. For example:

```shell
OFFLINE=1 make generate
```

### Run the site locally using Docker

> Note: running with docker only uses the Hugo server and not Netlify locally.
  This means that redirects and other Netlify features the site relies on will not work.
  It will also not include the processing required for live code blocks
  to show up correctly.

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
- [NodeJS](https://nodejs.org) (and NPM)

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

To check the site's links, first start the full site preview locally (see [Serving the full site](#serving-the-full-site) instructions))

Then run:

```bash
docker run --rm -it --net=host linkchecker/linkchecker $URL
```

Note: You may need to adjust the `URL` (host and/or port) depending on the environment. For OSX
and Windows the host might need to be `host.docker.internal` instead of `localhost`.

> This link checker will work on best with Netlify previews! Just point it at the preview URL instead of the local server.
  The "pretty url" feature seems to work best when deployed, running locally may result in erroneous links.

## Live Code Blocks

Live blocks enable readers to interact with and change Rego snippets from within the docs.
They are written as standard markdown code blocks with a special syntax in the language label
and are enabled used a postprocessing step during full site builds.

> For them to render correctly you must build the site using `make serve` or Netlify.
  If the live blocks build starts failing, try running `make live-blocks-clear-caches` from the `docs/` folder.
  For the page to be able to live update code blocks, you may also want to
  disable your browser's CORS checks when developing. For chrome you can use the command-line flags
  `--disable-web-security --user-data-dir=/tmp/$RANDOM`; this will open a new window
  that isn't tied to your existing user data.

Here's what they look like:

``````markdown
In this module:

```live:rule_body:module
package example

u {
  "foo" == "foo"
}
```

The rule `u` evaluates to true:

```live:rule_body:output
```
``````

### Groups

Each live block group within a page has a unique name, for example `rule_body` above.
Each group can be composed of up to 1 of each type of live block:

- `module` - A complete or partial rego module.
- `query` - Rego expressions for which to show output instead of the module.
- `input` - JSON input with which to evaluate the module/ query.
- `output` - A block to contain the output for the group, contents will be inserted automatically.

Groups can also be structured hierarchically using slashes in their names. When evaluated, the module blocks will be concatenated and any other missing blocks will be inherited from ancestors.
Here's what a more complex set of blocks could look like:

``````markdown
```live:eg:module:hidden
package example
```

We can define a scalar rule:

```live:eg/string:module
string = "hello"
```

Which generates the document you'd expect:

```live:eg/string:output
```

We can then define a rule that uses the value of `string`:

```live:eg/string/rule:module
r { input.value == string }
```

And query it with some input:

```live:eg/string/rule:input
{
  "value": "world"
}
```
```live:eg/string/rule:query:hidden
r
```

with which it will be undefined:

```live:eg/string/rule:output:expect_undefined
```
``````

In that example, the output for `eg/string` is evaluated with only the module:

```
package example

string = "hello"
```

Whereas the `eg/string/rule` output is evaluated with the module:

```
package example

string = "hello"

r { input.value == string }
```

as well the given query and input.

If any of the blocks that impact the output are edited by a reader,
the output will update accordingly

### Tags

You'll notice in the previous example that some blocks have an additional section after the type, these are tags.

> To apply multiple tags to a block, separate them with commas. E.g. `live:group_name:block_type:tag1,tag2,tag2`

Some tags can be applied to any block:

- `hidden` - Hide the code block.
- `read_only` - Prevent editing of the block.
- `merge_down` - Visually merge this code block with the one below it (remove bottom margin).
- `openable` - Add a button to the block that opens its group in the Rego Playground. This should typically
  only be used on complete module blocks.
- `line_numbers` - Show line numbers in the block. This should be used sparingly; the code will visually shift when they appear.

Outputs can also be tagged as expecting various errors. If one is tagged as expecting errors that do not occur
or errors occur that it is not tagged as expecting, the build will fail noisily.

More specific errors appear before less specific ones in this list, try to use them.
If you think a more specific tag could be added for your case, please create an issue.

- `expect_undefined` - The query result is undefined. If all the rules in a module are undefined, the output is simply `{}`.
- `expect_assigned_above` - A variable is already `:=` assigned.
- `expect_referenced_above` - A variable is used before it is `:=` assigned.
- `expect_compile_error`
- `expect_rego_type_error` - A compile-time type error.
- `expect_unsafe_var`
- `expect_recursion`
- `expect_rego_error` - Any parse/compile-time error.
- `expect_conflict`
- `expect_eval_type_error` - An evaluation-time type error.
- `expect_builtin_error`
- `expect_with_merge_error`
- `expect_eval_error` - Any evaluation-time error.
- `expect_error` - Any OPA error. This should generally not be used.

> At this time, due to the way OPA loads modules on the CLI, expecting parse errors is not possible.

Finally, outputs can also be tagged one or more times with `include(<group name>)`, which will include
another group's module when evaluating (e.g. so that they can be imported).

> If a query isn't specified for the output's group, when other modules are included the default becomes `data` instead of `data.<package name>`.