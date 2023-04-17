import 'core-js/stable'
import 'regenerator-runtime/runtime'

import CodeMirror from 'codemirror'
// eslint-disable-next-line
import 'codemirror/lib/codemirror.css'
import 'codemirror/mode/javascript/javascript'

import './style.css'
import {BASE_EDITOR_OPTS, BLOCK_SELECTOR, BLOCK_TYPES, CLASSES, EDITOR_MODES, HYDRATION_QUEUE_SORT_PERIOD, ICONS, INTERACTIVE_PATH, LINE_NUMBERS_EDITOR_OPTS, OPENING_IN_PLAYGROUND_PATH, READ_ONLY_EDITOR_OPTS, STATIC_TAG_TYPES} from '../constants'
import {batchProcess, delay, getAllGroupModules, getGroupField, handleLater, includedGroupNames, infoFromLabel, report} from '../helpers'
import {OPAErrors} from '../errors'

import {playgroundEval, shareToPlayground} from './playground'

import 'codemirror-rego/key-map'
import 'codemirror-rego/mode'
import 'codemirror/addon/comment/comment'
import 'codemirror/addon/edit/matchbrackets'

// --- MAIN ---
start() // Enter async.
async function start() {
  const codeElts = document.querySelectorAll(BLOCK_SELECTOR)
  if (!codeElts.length) {
    return // Exit! No live blocks.
  }

  const groups = constructGroups(codeElts)
  registerChangeHandlers(groups)
  await hydrate(groups)
}

// --- HYDRATION HELPERS ---
// Constructs an object of the following form:
// groupName: {
//   blockType (module/query/input/output): {
//     tags: [string],
//     get: () => string,
//     set: (string value[, boolean error]) => undefined,
//     restore: () => undefined // Restore the contents and error-state of the container.
//     changeHandlers: [0+ functions],
//     codeElt,
//     iconBar,
//     container,
//     codeMirror: initially, sometimes always, undefined
//     openButton: initially, sometimes always, undefined
//   },
//   output: {
//     ... same as above
//     current: initially undefined // Used to discard out of date responses from the playground
//     timeout: initially undefined // Used to debounce changes to dependencies
//   }
// }
function constructGroups(codeElts) {
  const groups = {}

  for (let codeElt of codeElts) {
    const container = codeElt.parentNode.parentNode

    const label = codeElt.dataset.lang
    let info = infoFromLabel(label)

    if (!groups[info.group]) { // Populate second level of the groups object structure
      groups[info.group] = {}
    }

    const initText = codeElt.textContent
    const initErr = container.classList.contains(CLASSES.BLOCK_CONTAINER_ERROR)
    groups[info.group][info.type] = {
      tags: info.tags,
      get: () => codeElt.textContent,
      set: (value, error) => {
        codeElt.textContent = value
        if (typeof error === 'boolean') {
          if (error) {
            container.classList.add(CLASSES.BLOCK_CONTAINER_ERROR)
          } else {
            container.classList.remove(CLASSES.BLOCK_CONTAINER_ERROR)
          }
        }
        runHandlers(groups[info.group][info.type].changeHandlers)
      },
      restore: () => {
        groups[info.group][info.type].set(initText, initErr)
      },
      changeHandlers: [],
      codeElt,
      iconBar: container.firstElementChild.firstElementChild,
      container,
    }
  }

  return groups
}

function registerChangeHandlers(groups) {
  for (let [groupName, group] of Object.entries(groups)) {
    const output = group.output
    if (!output) { // Not all groups have outputs
      continue
    }
    // Output contents depend on non-undefined query and input blocks in this group/a parent, this group's chain of inherited module blocks, as well as any included module blocks.
    const dependencies = [BLOCK_TYPES.QUERY, BLOCK_TYPES.INPUT].map((type) => getGroupField(groups, groupName, type)).filter((block) => !!block)
    dependencies.push(...getAllGroupModules(groups, groupName))
    dependencies.push(...(includedGroupNames(output.tags).reduce((iMs, iGN) => iMs.concat(getAllGroupModules(groups, iGN)), [])))

    for (let dependency of dependencies) {
      dependency.changeHandlers.push(() => updateOutput(groups, groupName, output))
    }
  }
}

// Hydrate the all the blocks as applicable
async function hydrate(groups) {
  let queue = Object.entries(groups).reduce((arr, [groupName, group]) => arr.concat(Object.entries(group).map(([type, block]) => ({groupName, group, type, block}))), [])
  function sort() { // Sort based on distance to the top of the screen
    queue.sort((a, b) => Math.abs(b.block.container.getBoundingClientRect().top) - Math.abs(a.block.container.getBoundingClientRect().top))
  }
  sort()

  for (let next = queue.pop(); next; next = queue.pop()) {
    const {groupName, type, block} = next

    // Hydrate
    hydrateEditor(type, block)
    hydrateButtons(groups, groupName, type, block)

    // Occasionally re-sort the queue in case the user scrolls.
    if (!(queue.length % HYDRATION_QUEUE_SORT_PERIOD)) {
      sort()
    }

    // Don't freeze the browser :)
    await delay(1)
  }
}

// Hydrate visible editors
function hydrateEditor(type, block) {
  if (!block.tags.includes(STATIC_TAG_TYPES.HIDDEN)) { // Only bother adding the editor to visible blocks
    // Replace intermediate container and hydrate it
    const cmDiv = document.createElement('div')
    block.codeElt.parentNode.replaceWith(cmDiv)
    const codeMirror = CodeMirror(cmDiv, Object.assign({
      value: block.get()
    }, constructEditorOptions(type, block.tags)))
    fixScroll(codeMirror)
    codeMirror.on('change', () => {
      runHandlers(block.changeHandlers)
    })

    // Add class to trigger shadows
    if (isEditable(type, block.tags)) {
      block.container.classList.add(CLASSES.BLOCK_CONTAINER_EDITABLE)
    }

    // Update block object
    Object.assign(block, {
      get: () => codeMirror.getValue(),
      set: (value, error) => {
        codeMirror.setValue(value)
        if (typeof error === 'boolean') {
          if (error) {
            block.container.classList.add(CLASSES.BLOCK_CONTAINER_ERROR)
          } else {
            block.container.classList.remove(CLASSES.BLOCK_CONTAINER_ERROR)
          }
        }
        fixScroll(codeMirror)
      },
      codeMirror
    })
  }
}

// Hydrate the restore and share buttons as applicable
function hydrateButtons(groups, groupName, type, block) {
  if (isEditable(type, block.tags)) {
    const restoreButton = createButton(ICONS.RESTORE)
    restoreButton.addEventListener('click', () => block.restore())
    block.iconBar.appendChild(restoreButton)
  }

  if (isInteractive(block.tags) && block.tags.includes(STATIC_TAG_TYPES.OPENABLE)) {
    // The three versions of the "Open in Playground" button.
    const readyButton = createButton(ICONS.PLAYGROUND.READY)
    const workingButton = createButton(ICONS.PLAYGROUND.WORKING)
    const errorButton = createButton(ICONS.PLAYGROUND.ERROR)

    // Handler to open in the playground.
    const open = createOpenInPlaygroundHandler(groups, groupName, block, readyButton, workingButton, errorButton)

    // Only ready and error trigger an open, if "working", forces user to wait until it completes.
    readyButton.addEventListener('click', open)
    errorButton.addEventListener('click', open)

    // Hydrate
    putOpenInPlaygroundButton(block, readyButton)
  }
}

// Create a new button in the DOM based on the given icon constant.
function createButton(icon) {
  const image = document.createElement('img')
  image.src = icon.src
  image.classList.add(CLASSES.ICON_IMG)
  if (icon.spinning) {
    image.classList.add(CLASSES.ICON_IMG_SPINNING)
  }
  const container = document.createElement('div')
  container.title = icon.title
  container.classList.add(CLASSES.ICON_CONTAINER)
  container.classList.add(CLASSES.ICON_CONTAINER_BUTTON)
  container.appendChild(image)
  return container
}

// Update the contents of an output block from a group.
function updateOutput(groups, groupName, output) {
  output.set('...' + '\n'.repeat((output.get().match(/\n/g) || []).length), false) // Show that it's in progress without changing size

  clearTimeout(output.timeout) // Debounce: If update is called rapidly, cancel last one.

  output.timeout = setTimeout((async () => {
    const promise = handleLater(() => playgroundEval(groups, groupName)) // Wrap with handleLater so that immediate rejection isn't a problem
    output.current = promise // Store the promise to the playground's response globally (tied to this output)

    const [text, error] = await promise // Time passes...

    if (output.current === promise) { // Is the global current promise (for this output block) still the one created by this call?
      if (!error) {
        output.set(text, false)
      } else {
        if (!(error instanceof OPAErrors) || !error.matchesExpected(output.tags)) {
          report(error) // Some extra logging for unexpected errors.
        }
        output.set(error.message, true)
      }
    } else { // A new request was made while waiting for this response, discard it.
      return
    }
  }), 500) // Wait 0.5s before fetching in case they're still typing, will be canceled above if they do.
}

// Padding was causing issues where text on the right side got cut off, this fixes it for a given CodeMirror instance.
function fixScroll(editor) {
  // Below is copied from the CodeMirror window resize handler, codeMirror.refresh() didn't work...
  const d = editor.display
  d.cachedCharWidth = d.cachedTextHeight = d.cachedPaddingH = null
  d.scrollbarsClipped = false
  editor.setSize()
}

// Runs every potentially async handler of handlers, logging unexpected errors. Guaranteed to not reject so can be run without catching.
async function runHandlers(handlers) {
  try {
    await batchProcess(0, handlers)
  } catch (errs) {
    report('one or more errors occurred in change handlers', handlers, errs)
  }
}

// Determines whether a block can be interacted with (a lower bar than editability) based on its tags and the current page's path.
function isInteractive(tags) {
  if (tags.includes(STATIC_TAG_TYPES.HIDDEN)) {
    return false
  }
  return INTERACTIVE_PATH.test(window.location.pathname);

}

// Determines whether a block should be editable based on its type, tags, and the current page's path.
function isEditable(type, tags) {
  if (!isInteractive(tags)) {
    return false
  }
  if (type === BLOCK_TYPES.OUTPUT) {
    return false
  }
  if (tags.includes(STATIC_TAG_TYPES.READ_ONLY)) {
    return false
  }
  return true
}

// Returns the options to use with a given block type that has the given tags.
function constructEditorOptions(type, tags) {
  const out = Object.assign({}, BASE_EDITOR_OPTS)

  out.mode = EDITOR_MODES[type]

  if (!isEditable(type, tags)) {
    Object.assign(out, READ_ONLY_EDITOR_OPTS)
  }

  if (tags.includes(STATIC_TAG_TYPES.LINE_NUMBERS)) {
    Object.assign(out, LINE_NUMBERS_EDITOR_OPTS)
  }

  return out
}

// Add/ replace the "Open in Playground" button.
function putOpenInPlaygroundButton(block, button) {
  if (!block.openButton) {
    block.iconBar.appendChild(button)
  } else {
    block.openButton.replaceWith(button)
  }
  block.openButton = button
}

// Returns a function that when called attempts to open the given group in the playground. Puts the appropriate button for the given block as it works.
// NOTE: The user should not be able to trigger the returned function when the button is set to `working`.
function createOpenInPlaygroundHandler(groups, groupName, block, ready, working, error) {
  return function () {
    // Open window (really a tab) immediately in this bog-standard, non-async, non-arrow handler to minimize the chance that browsers will block it/ ask permission.
    const pg = window.open(OPENING_IN_PLAYGROUND_PATH, '_blank')

    try {
      // Show that it's working
      putOpenInPlaygroundButton(block, working)

      // Try to maintain focus, this generally won't work due to malicious popup mitigations in browsers.
      pg.blur()
      window.focus()
    } catch (e) {
      report(e)
      // If any of those not only didn't work but actually errored, it's still not a problem. Continue...
    }

    // Asyncronously try to perform the share and update the new window accordingly.
    (async () => {
      try {
        const url = await shareToPlayground(groups, groupName)
        pg.location.replace(url)
        pg.focus()
        putOpenInPlaygroundButton(block, ready)
      } catch (e) {
        report('unable to open in playground', e)
        pg.close()
        putOpenInPlaygroundButton(block, error)
      }
    })()
  }
}