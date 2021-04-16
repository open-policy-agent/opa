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

[website/data/integrations.yaml](./website/data/integrations.yaml) - Source for the
integrations index. See [Integration Index](#integration-index) below for more details.

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
changes in this directory are pushed to `master`, the site will be re-built and
re-deployed.

## How to Edit and Test
### Preview Markdown `content` (*.md)

The majority of this can be done with any markdown renderer (typically built into or
a via plug-in for IDE's and editors). The rendered output will be very similar to what Hugo will
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
   CLI console output.


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

# Integration Index

The integration index makes it easy to find either a specific integration with OPA 
or to browse the integrations with OPA within a particular category.  And it pulls 
information about that integration (e.g. blogs, videos, tutorials, code) into a 
single place while allowing integration authors to maintain the code wherever they like.  

## Schema

The schema of integrations.yaml has the following highlevel entries, each of which is self-explanatory.
* integrations
* organizations
* software

Each entry is an object where keys are unique identifiers for each subentry.  
Organizations and Software are self-explanatory by inspection.  The schema for integrations is as follows.

* title: string
* description: string
* software: array of strings
* labels: collection of key/value pairs.
* tutorials: array of links
* code: array of links
* inventors: array of either
  * string (organization name)
  * object with fields
    * name: string
    * organization: string
* videos: array of either
  * link
  * object with fields
    * title: string
    * speakers: array of name/organization objects
    * venue: string
    * link: string
* blogs: array of links

The UI for this is currently hosted at [https://openpolicyagent.org/docs/latest/ecosystem/](https://openpolicyagent.org/docs/latest/ecosystem/)

The future plan is to use the following labels to generate categories of integrations.

* layer: which layer of the stack does this belong to
* category: which kind of component within that layer is this
* type: what kind of integration this is.  Either `enforcement` or `poweredbyopa`.  `enforcement` is the default 
  if `type` is missing.  `poweredbyopa` is intended to be integrations built using OPA that are not tied to a 
  particular layer of the stack.  This distinction is the most ambiguous and may change.

As of now the labels are only displayed for each entry.

## Logos
For each entry in the [integrations.yaml](./website/data/integrations.yaml) integrations section the UI will use a
PNG logo with the same name as the key from [./website/static/img/logos/integrations](./website/static/img/logos/integrations)

For example:

```yaml
integrations:
  my-cool-integration:
    ...
```

Would need a file called `my-cool-integration.png` at `./website/static/img/logos/integrations/my-cool-integration.png`

If it doesn't exist the OPA logo will be shown by default.
