import process from 'process'
import {promises as promFS} from 'fs'

import * as filepath from 'path';

import cheerio from 'cheerio'

import {batchMapFold, batchProcess, expectedErrorTags, infoFromLabel, println, report, toArray} from '../helpers.js'
import {BLOCK_SELECTOR, BLOCK_TYPES, CLASSES, CSS_BUNDLE_BATH, ICONS, JS_BUNDLE_PATH, MAX_CONCUR_FILE_EVALS, MAX_CONCUR_FILES, STATIC_TAG_TYPES, VERSION_EDGE, VERSION_LATEST} from '../constants.js'
import {ChainedError, OPAErrors} from '../errors.js'

import localEval from './localEval.js'

// --- MAIN ---
start() // Enter async.
async function start() {
  // Process the files, generating a map from paths that resulted in errors to an array of those errors.
  // This allows several files to be processed at once, without processing ALL of them at the same time, while reporting as many errors as possible so that they can be fixed.
  const errors = await batchMapFold(MAX_CONCUR_FILES, process.argv.slice(2), processFile, (output, processingResult) => {
    const {input: path, output: [_, err]} = processingResult
    if (err) {
      output[path] = toArray(err)
    }
    return output
  }, {})

  if (Object.keys(errors).length) { // Any of the files have errors
    for (let path in errors) { // Log details (stack trace)
      report(`\n--- Errors for ${path} ---`)
      for (let error of errors[path]) {
        report(error)
      }
    }
    report('\n\n')

    report("It looks like there were some errors. Here's an overview:")
    for (let path in errors) { // Log just the messages (probably the only thing needed most of the time)
      report(`\n${path}:`)
      for (let error of errors[path]) {
        report(`\t${error.message}`)
      }
    }
    report('\n\n')

    process.exit(1) // Exit with non-zero code to prevent the docs from being deployed.
  } else {
    // All good!
    println('Live docs preprocessing complete!')
    process.exit(0)
  }
}

// Processes a file at a path, may reject with one or more errors (the latter as an array) that occurred while trying to do so.
async function processFile(path) {
  const version = getVersion(path)

  if (!version) {
    return // Skip! No version.
  }

  const $ = cheerio.load(await promFS.readFile(path))

  const codeElts = $(BLOCK_SELECTOR)

  if (!codeElts.length) {
    return // Skip! No live blocks.
  }

  println(`Processing ${path}...`)
  const groups = constructGroups($, codeElts)
  styleBlocks(groups)
  await populateOutputs(groups, version)
  enliven($)
  await promFS.writeFile(path, $.html())
}

// --- PROCESSING HELPERS ---
// Based on a file's path containing /v[VERSION]/ or /edge/ returns the version string (e.g. '0.12.1' or 'edge'). Returns latest if it can't find a version.
function getVersion(path) {
  const versionMatch = (new RegExp(`/(v[^/]*|${VERSION_EDGE}|${VERSION_LATEST})/`)).exec(filepath.resolve(path))
  if (!versionMatch) {
    if (process.env.LATEST) {
      return VERSION_LATEST
    }
    return VERSION_EDGE
  }

  return versionMatch[1]
}

// Constructs an object of the following form, may throw an array of errors.
// {
//   groupName: {
//     module/query/input/output: {
//       tags: [string],
//       get: () => string,
//       set: (string value[, boolean error]) => undefined
//       codeElt: cheerio code block
//       container: cheerio element
//     }
//   }
// }
function constructGroups($, codeElts) {
  const groups = {}
  const errors = []
  codeElts.each(function () { // Cheerio's `each` method (codeElts is not iterable).
    try {
      const codeElt = $(this)
      // remove trailing newline
      codeElt.text(codeElt.text().trim())

      // wrap in div
      const container = $('<div></div>').addClass(CLASSES.BLOCK_CONTAINER)
      codeElt.parent().wrap(container)

      const label = codeElt.data('lang')
      let info = infoFromLabel(label)

      // Populate second level of the groups object structure
      if (!groups[info.group]) {
        groups[info.group] = {}
      }

      // Prevent duplicates at preprocess-time
      if (info.type in groups[info.group]) {
        throw new Error(`${info.group} cannot have more than one ${info.type}`)
      }

      // Prevent non-empty outputs
      if (info.type === BLOCK_TYPES.OUTPUT && !(/^\s*$/).test(codeElt.text())) {
        throw new Error(`${info.group}'s output must not contain anything; it will be populated automatically`)
      }

      groups[info.group][info.type] = {
        tags: info.tags,
        get: () => codeElt.text(),
        set: (value, error) => {
          codeElt.text(value)
          if (typeof error === 'boolean') {
            if (error) {
              container.addClass(CLASSES.BLOCK_CONTAINER_ERROR)
            } else {
              container.removeClass(CLASSES.BLOCK_CONTAINER_ERROR)
            }
          }
        },
        codeElt,
        container,
      }
    } catch (e) {
      errors.push(e)
    }
  })
  if (errors.length) {
    throw errors
  } else {
    return groups
  }
}

// Add some classes, styles, and elements to the blocks/ their containers. Generally shouldn't but may throw an error.
function styleBlocks(groups) {
  for (let group of Object.values(groups)) {
    for (let [type, block] of Object.entries(group)) {
      // Reset highlighting
      const content = block.get()
      block.codeElt.empty()
      block.set(content)

      // Hide if tagged
      if (block.tags.includes(STATIC_TAG_TYPES.HIDDEN)) {
        block.container.css('display', 'none')
      }

      // Merge if tagged
      if (block.tags.includes(STATIC_TAG_TYPES.MERGE_DOWN)) {
        block.container.addClass(CLASSES.BLOCK_CONTAINER_MERGED_DOWN)
      }

      // Add icon bar/ type mark
      block.container.prepend(`
<div class="${CLASSES.ICON_BAR_OUTER}">
  <div class="${CLASSES.ICON_BAR_INNER}">
    <div class="${CLASSES.ICON_CONTAINER} ${CLASSES.ICON_CONTAINER_INDICATOR}" title="${ICONS[type].title}">
      <img class="${CLASSES.ICON_IMG}" src="${ICONS[type].src}">
    </div>
  </div>
</div>
      `)
    }
  }
}

// Evaluate what the output blocks should contain and set it. May throw an array of errors if the evaluations can't be performed or they result in unexpected errors.
async function populateOutputs(groups, opaVersion) {
  await batchProcess(MAX_CONCUR_FILE_EVALS, Object.entries(groups).filter(([, group]) => {
    return BLOCK_TYPES.OUTPUT in group // Only process groups that have an output (some parent groups, e.g. that others inherit module fragments from, may not have one)

  }).map(([groupName, group]) => (async () => {
    const block = group[BLOCK_TYPES.OUTPUT]
    const expectedETags = expectedErrorTags(block.tags)
    const strETags = JSON.stringify(expectedETags)

    let resultString
    try {
      resultString = await localEval(groups, groupName, opaVersion)
    } catch (e) { // If there's an error it might be expected (e.g. syntax errors like unsafe vars)
      if (e instanceof OPAErrors) { // Are the errors from OPA?
        if (e.matchesExpected(block.tags)) {
          block.set(e.message, true)
          return // Output populated, exit!
        }
        throw new ChainedError(`${groupName} evaluation failed unexpectedly (${expectedETags.length ? `only tagged with: ${strETags}` : 'untagged'}):\n${e.message}\n`, e)

      }
      // Errors are not from OPA, something bad happened.
      throw new ChainedError(`${groupName} evaluation failed for an unexpected, possibly internal reason (check for additional logs): ${e.message}`, e)
    }

    // If there's not an error, one might have been expected.
    if (expectedETags.length) {
      throw new Error(`${groupName} is tagged with ${strETags} but evaluation result was:\n${resultString}\n`)
    } else {
      block.set(resultString, false)
    }
  })))
}

// Add the JS and CSS elements to the body/ head for hydrating the code blocks.
function enliven($) {
  const head = $('head').first()
  const body = $('body').first()
  if (!head) {
    throw new Error('no head element')
  }
  if (!body) {
    throw new Error('no body element')
  }

  // Add to beginning of head to allow the page to override style in other
  // css sources later on in <head>.
  head.prepend(`<link rel="stylesheet" href="${CSS_BUNDLE_BATH}">`)

  body.append(`<script src="${JS_BUNDLE_PATH}"></script>`)
}