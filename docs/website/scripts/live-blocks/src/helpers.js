import {BLOCK_TYPES, EVAL_CONFIG_TAG_TYPES, EXPECTED_ERROR_TAG_TYPES, LABEL_REG_EXP} from './constants'
import {ChainedError} from './errors'

// --- LABELS ---
// Takes a string that can be formatted like live:GROUP:BLOCK_TYPE[:TAG_TYPE[,TAG_TYPE[...]]], returns {group: /\w+/, type: BLOCK_TYPES[?], tags: [ string ]}.
// Throws an error if it looks like a live code block but the label can't be parsed.
export function infoFromLabel(label) {
  const info = LABEL_REG_EXP.exec(label)
  if (!info) {
    throw new Error(`improperly formated live code block label: ${label}`)
  }

  let tags = []
  if (info[3].length > 1) {// Will include ':', minimum if there are tags is 2 chars
    tags = info[3].substring(1).split(',')
  }

  // Validate parsing
  if (info[2] !== BLOCK_TYPES.OUTPUT && expectedErrorTags(tags).length) { // Only outputs can expect errors
    throw new Error(`${info[2]}s cannot expect errors (only outputs): ${label}`)
  }
  if (info[2] !== BLOCK_TYPES.OUTPUT && includedGroupNames(tags).length) { // Only outputs can include groups for evaluation
    throw new Error(`${info[2]}s cannot include modules for evaluation (only outputs): ${label}`)
  }

  // Looks good
  return {
    group: info[1],
    type: info[2],
    tags
  }
}

// Returns an array of the expected error tag strings in the provided array of tags
export function expectedErrorTags(tags) {
  const errors = Object.values(EXPECTED_ERROR_TAG_TYPES)
  return tags.filter((tag) => errors.includes(tag))
}

// Returns an array of the included group names from `include(...)` tags in the provided array.
export function includedGroupNames(tags) {
  return tags.reduce((iGNs, t) => {
    const res = EVAL_CONFIG_TAG_TYPES.INCLUDE.exec(t)
    if (res) {
      iGNs.push(res)
    }
    return iGNs
  }, [])
}

// --- GROUPS AND BLOCKS ---
// Gets the value of the deepest field with a given name in the group/ parent groups, otherwise undefined.
export function getGroupField(groups, groupName, field) {
  const nameFragments = groupName.split('/') // Fragments of the group name

  while (nameFragments.length) { // Pop off fragments until the field is set
    const group = groups[nameFragments.join('/')]
    if (group && field in group) {
      return group[field]
    }
    nameFragments.pop()
  }

  return undefined // Explicit: it failed
}

// Gets an array of module blocks from group and/or its parents from shallowest to deepest
export function getAllGroupModules(groups, groupName) {
  const out = []

  const nameFragments = groupName.split('/') // Fragments of the group name
  while (nameFragments.length) { // Append module blocks to out, pop off fragments
    const group = groups[nameFragments.join('/')]
    if (group && BLOCK_TYPES.MODULE in group) {
      out.push(group[BLOCK_TYPES.MODULE])
    }
    nameFragments.pop()
  }

  return out.reverse() // Correct the order
}

// Returns an object of the form
//
// {
//    module: string,
//    package: string
//    input: value
//    query: string,
//    included: map[filename]string
// }
//
// Throws an error with a human-readable message if the specified or any
// included groups don't have modules, the group's package cannot be found, or
// if the input cannot be parsed.
export function getGroupData(groups, groupName) {
  const queryBlock = getGroupField(groups, groupName, BLOCK_TYPES.QUERY)
  const inputBlock = getGroupField(groups, groupName, BLOCK_TYPES.INPUT)

  const out = {module: getCompleteGroupModule(groups, groupName)}

  let pkgRegexRes = /^package[ \t]+([\w.-]+)[ \t]*#*.*$/m.exec(out.module)
  if (!pkgRegexRes) {
    throw new Error('couldn\'t find package declaration in module')
  }
  out.package = pkgRegexRes[1]

  if (queryBlock) {
    out.query = queryBlock.get()
  }
  if (inputBlock) {
    try {
      out.input = JSON.parse(inputBlock.get())
    } catch (e) {
      throw new Error('can\'t parse input', e)
    }
  }
  if (groupName in groups && BLOCK_TYPES.OUTPUT in groups[groupName]) {
    const included = includedGroupNames(groups[groupName][BLOCK_TYPES.OUTPUT].tags)
    if (included.length) {
      out.included = {}
      for (let name of included) {
        try {
          out.included[`${name}.rego`] = getCompleteGroupModule(groups, name)
        } catch (e) {
          throw new ChainedError(`unable to include ${name}: ${e.message}`, e)
        }
      }
    }
  }
  return out
}

// Returns the string of the full group's module or throws a user friendly error if there are no module blocks.
function getCompleteGroupModule(groups, groupName) {
  const moduleBlocks = getAllGroupModules(groups, groupName)

  if (!moduleBlocks.length) {
    throw new Error('no module')
  }

  return moduleBlocks.map((block) => block.get()).join('\n\n')
}

// --- ASYNC ---
// Returns a promise that resolves to an array of the form [returned, thrown] (at least one will be undefined) where they are the result of awaiting a call to the passed function.
// This is useful for awaiting an operation now but handling its results later and/or handling it multiple times.
// WARNING: Make sure thrown values get handled, otherwise they'll get swallowed. This is effectively a work-around for unhandled rejection warnings/errors.
export async function handleLater(func) {
  try {
    return [await func(), undefined]
  } catch (e) {
    return [undefined, e]
  }
}

// Runs an array of async functions (`jobs`) maxConcurrent at a time, these can each reject with one or more errors (the latter as an array).
// Once they've all finished, either resolves to an array of results ordered by when the jobs completed or rejects with a similarly ordered, potentially differently sized array of errors.
// maxConcurrent is treated as unlimited if < 1
export async function batchProcess(maxConcurrent, jobs) {
  if (maxConcurrent < 1 || maxConcurrent > jobs.length) {
    maxConcurrent = jobs.length // Run all of them simultaneously if maxConcurrent is beyond reasonable values
  }
  jobs = Array.from(jobs) // Shallow copy, the new array will get mutated

  const out = []
  const errs = []
  async function process() {
    for (let job = jobs.pop(); job; job = jobs.pop()) { // Pop off jobs
      try {
        out.push(await job())
      } catch (e) {
        errs.push(...toArray(e))
      }
    }
  }
  await Promise.all(Array.from({length: maxConcurrent}, () => process())) // Run maxConcurrent calls to process() asyncronously

  if (errs.length) {
    throw errs
  } else {
    return out
  }
}

// Runs an array of `inputs` through the async function `map`.
// Each time it determines it's new output (returned when all inputs are processed) by calling fold with the current output (starting with the value outBase) and map result ({input: value, output: [returned, thrown]}).
// maxConcurrent is passed to the underlying call to `batchProcess`.
export async function batchMapFold(maxConcurrent, inputs, map, fold, outBase) {
  await batchProcess(maxConcurrent, inputs.map((input) => async () => { // Create an array of async jobs to map and fold each input
    const result = {
      input,
      output: await handleLater(() => map(input))
    }
    outBase = fold(outBase, result) // Folding happens syncronously (no awaits) so there's no concurrency concerns
  }))
  return outBase
}

// Returns a promise that resolves in the given number of milliseconds.
export function delay(ms) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms)
  })
}

// --- MISC ---
// Wraps a value in an array if it isn't one already
export function toArray(val) {
  if (Array.isArray(val)) {
    return val
  }
  return [val]
}

// Aliases for console functions so that messages can be intercepted in the future and stray debugging statements are easier to find.
export const {log: println, error: report} = console