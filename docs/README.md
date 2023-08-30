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

[content/integrations/](./content/integrations) - Source for the
OPA Ecosystem page. See [OPA Ecosystem](#opa-ecosystem) below for more details.

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
which are defined in [website/layouts/index.redirects](./website/layouts/index.redirects)
and are build into a `_redirects` file when the Hugo build happens via
`make production-build` or `make preview-build`.

## Site updates

The OPA site is automatically published using [Netlify](https://netlify.com). Whenever
changes in this directory are pushed to `main`, the site will be re-built and
re-deployed.

**Note:** The site is built for many versions of the docs, this introduces some
complexities to be aware of when making changes to the site's layout:

* Updates to the [site's templates or styles/](./website/) are applied to all versions
  (edge, latest and all versions) when merged to `main`.
* Site [data](./website/data) treated in the same way, so updates to data files also
  apply to all versions as soon as they are merged.
* Docs [content/](./content/), when merged to `main`, is only shown on `edge` until the
  next release.
* Other, unversioned [content/](./website/content/) is shown immediately after merging.
  This includes pages in the [OPA Ecosystem](https://www.openpolicyagent.org/ecosystem/)
  as well as [Security](https://www.openpolicyagent.org/security/),
  [Support](https://www.openpolicyagent.org/support/), and
  [Community](https://www.openpolicyagent.org/community/) pages.

## How to Edit and Test

### Preview Markdown `content` (*.md)

The majority of this can be done with any markdown renderer (typically built into or
via a plug-in for IDEs and editors). The rendered output will be very similar to what Hugo will
generate.

> This excludes the Hugo shortcodes (places with `{{< SHORT_CODE >}}` in the markdown.
  To see the output of these you'll need to involve Hugo. Additionally, to validate
  and view live code blocks, a full site preview is required.

### Full Site Preview

To build and serve the site install the following dependencies
on your system:

- The [Hugo](#installing-hugo) static site generator (See details below)
- [NodeJS](https://nodejs.org) (and NPM)
- The [Netlify dev CLI](https://www.netlify.com/products/dev/)

#### Installing Hugo

Running the website locally requires installing the [Hugo](https://gohugo.io) static
site generator. The required version of Hugo is listed in the
[`netlify.toml`](../netlify.toml) configuration file (see the `HUGO_VERSION` variable).

Installation instructions for Hugo can be found in the [official
documentation](https://gohugo.io/getting-started/installing/).

Please note that you need to install the "extended" version of Hugo (with built-in
support) to run the site locally. If you get errors like this, it means that you're
using the non-extended version:

```
error: failed to transform resource: TOCSS: failed to transform "sass/style.sass" (text/x-sass): this feature is not available in your current Hugo version
```

Please also note that the current version of Hugo (e.g. installed with `brew install hugo`) is
not compatible with the docs. If you get errors like this, it means that you're using the
wrong version:

```
ERROR ... execute of template failed: template: partials/docs/sidenav.html:11:18: executing "partials/docs/sidenav.html" at <.URL>: can't evaluate field URL in type *hugolib.pageState
```

#### Remote Preview on Netlify

This option provides the best preview of the site content, using the exact same infrastructure as the production website.

1) Go to [Netlify](https://www.netlify.com/) and create an account/log-in.

1) (Optional) Create a new "site" in Netlify, linking to your fork of OPA on GitHub. This step makes for an easy way
   to see the "real" site deployed from branches you are working on, but is not required for dev previews.

1) Log in with the `netlify` CLI tool [https://docs.netlify.com/cli/get-started/#authentication](https://docs.netlify.com/cli/get-started/#authentication)

1) Deploy the site on Netlify using local content via the `make docs-serve-remote`. Follow any prompts the `netlify`
   tool asks. If you have not already linked the site select `Create & configure a new site` and specify your personal
   user account (which should have been configured in the previous step).

1) Netlify will then upload the built content and serve it via their CDN. A URL to the preview will be given in the
   CLI console output. Be sure to select the `edge` version within the Documentation pages to see your reflected changes in the remote site.

#### Local Preview via `netlify dev`

Similar to the "remote" option this will run a full preview of the website, however all Netlify components are run
locally using [Netlify Dev](https://www.netlify.com/products/dev/). This will load a preview significantly faster
than deploying remotely as the content does not need to be uploaded to the Netlify CDN first.

1) Run `make docs-serve-local` target. A URL to the preview will be given in the CLI console output.

> WARNING: This option will render the content and works as a good option for local development. However, there are
> some differences between what would be run locally and what gets deployed in "prod" on Netlify. This depends on
> the version of the CLI tool and how in-sync that is with the actual Netlify infrastructure. A common issue is
> with the redirects not working as expected locally, but working correctly on the "live" site in production.
> When in doubt use the "remote" Netlify preview to verify behavior.

## Checking links

To check the site's links, first start the full site preview locally (see [Serving the full site](#serving-the-full-site) instructions))

Then run:

```bash
docker run --rm -it --net=host linkchecker/linkchecker $URL
```

Note: You may need to adjust the `URL` (host and/or port) depending on the environment. For OSX
and Windows the host might need to be `host.docker.internal` instead of `localhost`.

> This link checker will work best with Netlify previews! Just point it at the preview URL instead of the local server.
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

# OPA Ecosystem

The [OPA Ecosystem](https://www.openpolicyagent.org/ecosystem/)
makes it easy to find either a specific integration with OPA
or to browse the integrations with OPA within a particular category. It pulls
information about different integrations (e.g. blogs, videos, tutorials, code) into a
single place while allowing integration authors to update the docs content as needed.

## Schema

Source information for the OPA Ecosystem is stored in the following places:

- [content/integrations/](./website/content/integrations) - each file creates a page in the OPA Ecosystem for a particular integration.
- [content/organizations/](./website/content/organizations) - each file is a page for organizations and companies associated with integrations.
- [content/softwares/](./website/content/softwares) - each file is for software categories related to integrations.

Integrations should have a file in `/docs/website/content/integrations/` with the following schema:

```md
---
title: <integration name>
software:
- <related software>
- <related software>
inventors:
- <inventor name>
- <inventor name>
tutorials: # optional, links to tutorials for the integration
- https://example.com/tutorial
code: # optional, links to code for the integration
- https://github.com/...
blogs: # optional, links to blog posts for the integration
- https://example.com/blog/1
videos: # optional, links to videos for the integration
- title: <video title>
  speakers:
  - name: <speaker name>
    organization: <speaker organization>
    venue: <venue>
    link: <link>
---
Description of the integration (required)
```

Any `inventor` that is not already in `content/organizations/` will need to be added too in `content/organizations/`.
Organizations have the following format:

```md
---
link: https://example.com
title: <organization name>
---
```

Any `software` that is not already in `content/softwares/` will need to be added too in `content/softwares/`.
Software categories have the following format:

```md
---
link: https://example.com
title: <software name>
---
```

## Logos

For each file in under [content/integrations/](./website/content/integrations) 
a png or svg logo with the same name must be placed in `./website/static/img/logos/integrations`.

For example:

```md
# content/integrations/my-cool-integration.md
```

Would need a file called `my-cool-integration.(png|svg)` at
`./website/static/img/logos/integrations/my-cool-integration.(png|svg)`.

## Google Analytics

With the addition of the feedback button, we are now able to see how many users found a particular page of the docs useful.

To view the metrics you will need access to [Google Analytics](https://analytics.google.com/analytics/web/).

Feedback responses can be found in the right hand tree under Behavior -> Events -> Top Events.

From here you can set the desired time frame you wish to monitor. Then drill down into the helpful category to see the specific pages and how many clicks they received.
