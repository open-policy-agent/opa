# Live Blocks

This is the high-level code documentation for the live blocks postprocessor.
Whoever you are, you should probably read the "Live Code Blocks" section of
`docs/README.md` first (it should be all you need if you just want to use
live blocks when writing docs).

`scripts/live-blocks/` acts as a completely standalone postprocessor, editing and
adding files in `docs/website/public` after the rest of a complete
build has finished and then hydrating the code blocks
on page load.

## Scripts
`package.json` contains a bunch of scripts, in some cases it calls things in the `live-blocks/scripts/` folder.
The most important of these is `inject` which calls a bunch of other scripts to add live docs functionality
to an otherwise fully built site. All of these scripts can be run from the `docs/` folder using `make live-blocks-<script name>`
or from the root of this repository using `make docs-live-blocks-<script name>`.

You probably want to run `install-deps` to automatically install JS dependencies. When testing, you might want to use something like
`make docs-live-blocks-clear-bundle-cache && make docs-serve` so that the bundles get re-built. If a cache gets corrupted, you can
run `clear-caches` or the script for a specific one to delete it. Once you're done, you can run `check`
to test and sort package.json.

## Folder Structure

- `dist/` (gitignored) - A local cache of bundled JS and CSS (automatically populated by `bundle-web`).
- `node_modules/` (gitignored) - A local cache of JS dependencies (automatically downloaded by `install-deps`).
- `opa_versions/` (gitignored) - A local cache of OPA binaries used for evaluating groups
  (automatically downloaded by `preprocess`; edge is redownloaded occasionally to keep it up-to-date).
- `scripts/` - Internal Bash and Node scripts to automate tasks. These should generally be called via `make` or `npm` as described above.
- `src/` - The live blocks source code.
  - `preprocess/` - Code that processes the documentation HTML before hydration.
    This includes evaluating the initial contents of output blocks.
    - `index.js` - Entry point.
  - `web/` - Code that hydrates and styles the live blocks once loaded by a reader.
    This includes reevaluating the contents of output blocks when their dependencies get edited.
    - `index.js` - Entry point.
  - `constants.js` - Various shared or platform-agnostic constant values (and some logic).
  - `helpers.js` - Various shared or platform-agnostic functions, particularly related to parsing block labels,
    working with groups, and async operations.
  - `errors.js` - A couple of shared error classes.
- `static/` - Static assets including the "Open In Playground" "working" page and icons.
- `test/` - Unit tests.
- `package.json` - NodeJS package file, including a few constants and scripts. All the scripts can be run via `make live-docs-` from the `docs/` folder.
- `rollup.config.js` - Configuration for the JS/CSS bundler.

## `groups` Object

On both the preprocessing and web sides, a `groups` object is constructed with mappings from full group names to group objects.
These group objects in turn map from block types to block objects. Between the two contexts these block objects have some matching and some additional fields,
their structure is documented by the respective `index.js`' `constructGroups()` functions.

## Preprocessing

The preprocessing stage tries to validate as much as it can, and fail noisily with as many of the problems
that need to be fixed as possible. Most errors will occur around either constructing the `groups` object or
evaluating the output contents, both steps will report all the errors for all the code blocks. One of these
sets of errors (or some other unexpected one) will be reported (if applicable) for every file. Note that,
in order to not store them while waiting for the rest to finish, files that are processed successfully are
written to disk even if others fail.

Preprocessing should happen as quickly as possible and so multiple files and/or group evaluations can be in the middle of processing,
waiting on file operations simultaneously.

## Hydration

Attaching the CodeMirror (editor) instances is easily the longest operation when hydrating the page
(on the order of 5-10 seconds for >100 live blocks).
As such, hydration happens in a couple of passes:

1.  The `groups` object is constructed.
2.  Dependencies are determined and change handlers are registered.
3.  The blocks are visibly hydrated based on how close they are to the viewport and the `groups` object
    is incrementally updated accordingly.

This allows almost immediate editing of visible live blocks by a reader without breaking (or leaving unupdated)
any dependent blocks whose editors haven't been hydrated yet.

## DOM Structure

Hugo generates code blocks like this:

```html
<div class="highlight">
  <pre style="background-color:#f8f8f8;-moz-tab-size:4;-o-tab-size:4;tab-size:4">
    <code class="language-python" data-lang="python">
      <!--text wrapped in spans to style them-->
    </code>
  </pre>
</div>
```

The preprocessor changes the div's class, adds an icon bar, and removes any highlighting from the code block:

```html
<div class="live--block-container">
  <div class="live--icon-bar--outer">
    <div class="live--icon-bar--inner">
      <div class="live--icon--container live--icon--container--indicator" title="Type">
        <img class="live--icon--img" src="/live-blocks/icons/type.svg">
      </div>
    </div>
  </div>
  <pre style="background-color:#f8f8f8;-moz-tab-size:4;-o-tab-size:4;tab-size:4">
    <code class="language-python" data-lang="python">
      <!--text-->
    </code>
  </pre>
</div>
```

On hydration, the rest of the Hugo-generated HTML is replaced by CodeMirror wrapped in a div:

```html
<div class="live--block-container">
  <div class="live--icon-bar--outer">
    <div class="live--icon-bar--inner">
      <div class="live--icon--container live--icon--container--indicator" title="Type">
        <img class="live--icon--img" src="/live-blocks/icons/type.svg">
      </div>
      <!--potentially more icons (buttons)-->
    </div>
  </div>
  <div>
    <div class="CodeMirror cm-s-default">
      <!--codemirror's internal structure-->
    </div>
  </div>
</div>
```

## Versioning

The preprocessor handles all doc versions the same, downloading the correct version of OPA
(based on the file's path) from either GitHub or S3 (the latter for current edge builds).

The web script always hydrates the blocks but only allows them to be edited if the page's path conforms
with `INTERACTIVE_PATH` in `constants.js`. This restricts the live functionality to the current version of the
Rego Playground.

## TODO

The `INTERACTIVE_PATH` should eventually only allow the current docs but currently it only allows edge.
Once doc updates are backported or there's a new OPA release, this will need to be updated in `src/constants.js`.