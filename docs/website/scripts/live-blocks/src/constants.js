// --- CSS CLASSES ---
// NOTE: Update style.css with any changes.
const join = (...fragments) => fragments.join('--')
const CLASS_PREFIX = 'live'
const BLOCK_CONTAINER = join(CLASS_PREFIX, 'block-container')
const ICON_BAR = join(CLASS_PREFIX, 'icon-bar')
const ICON = join(CLASS_PREFIX, 'icon')
const ICON_CONTAINER = join(ICON, 'container')
const ICON_IMG = join(ICON, 'img')
export const CLASSES = {
  BLOCK_CONTAINER,
  BLOCK_CONTAINER_EDITABLE: join(BLOCK_CONTAINER, 'editable'),
  BLOCK_CONTAINER_MERGED_DOWN: join(BLOCK_CONTAINER, 'merged-down'),
  BLOCK_CONTAINER_ERROR: join(BLOCK_CONTAINER, 'error'),
  HUGO_BLOCK_CONTAINER: 'highlight',
  ICON_BAR_OUTER: join(ICON_BAR, 'outer'),
  ICON_BAR_INNER: join(ICON_BAR, 'inner'),
  ICON_CONTAINER,
  ICON_CONTAINER_INDICATOR: join(ICON_CONTAINER, 'indicator'),
  ICON_CONTAINER_BUTTON: join(ICON_CONTAINER, 'button'),
  ICON_IMG,
  ICON_IMG_SPINNING: join(ICON_IMG, 'spinning')
}

// --- LABELS ---
// Prefix for "lang" labels for live code blocks
export const LABEL_PREFIX = 'live:'

// Regular expressions for recognizing group names.
const GROUP_NAME_FRAGMENT_REG_EXP = '\\w+'
const GROUP_NAME_REG_EXP = `${GROUP_NAME_FRAGMENT_REG_EXP}(?:\\/${GROUP_NAME_FRAGMENT_REG_EXP})*`

// The types of live code blocks with their names as found in labels
export const BLOCK_TYPES = {
  MODULE: 'module',
  QUERY: 'query',
  INPUT: 'input',
  OUTPUT: 'output',
}

// The types of expected error tags that output blocks can have with their names as found in labels
export const EXPECTED_ERROR_TAG_TYPES = {
  EXPECT_ERROR: 'expect_error',
  EXPECT_UNDEFINED: 'expect_undefined',
  // from 'ast'
  EXPECT_REGO_ERROR: 'expect_rego_error',
  // EXPECT_PARSE_ERROR: 'expect_parse_error', Not properly supported by the CLI.
  EXPECT_COMPILE_ERROR: 'expect_compile_error',
  EXPECT_REGO_TYPE_ERROR: 'expect_rego_type_error',
  EXPECT_UNSAFE_VAR: 'expect_unsafe_var',
  EXPECT_RECURSION: 'expect_recursion',
  // from 'topdown'
  EXPECT_EVAL_ERROR: 'expect_eval_error',
  EXPECT_CONFLICT: 'expect_conflict',
  EXPECT_EVAL_TYPE_ERROR: 'expect_eval_type_error',
  EXPECT_BUILTIN_ERROR: 'expect_builtin_error',
  EXPECT_WITH_MERGE_ERROR: 'expect_with_merge_error',
  // more specific
  EXPECT_ASSIGNED_ABOVE: 'expect_assigned_above',
  EXPECT_REFERENCED_ABOVE: 'expect_referenced_above',
}

// The types of eval configuration tags that output blocks can have with a `match` string used to build LABEL_REG_EXP and an `exec` function used to extract the value.
const config = (name, contents) => ({match: `${name}\\(${contents}\\)`, exec: (tag) => {
  const res = (new RegExp(`^${name}\\((${contents})\\)$`)).exec(tag)
  return res ? res[1] : res // get the captured value if it matches
}})
export const EVAL_CONFIG_TAG_TYPES = {
  INCLUDE: config('include', GROUP_NAME_REG_EXP),
  // NOTE: This could potentially include options for setting the query package/ imports.
}

// The non-variable types of tags that blocks can have with their names as found in labels.
export const STATIC_TAG_TYPES = {
  HIDDEN: 'hidden',
  READ_ONLY: 'read_only',
  MERGE_DOWN: 'merge_down',
  OPENABLE: 'openable',
  LINE_NUMBERS: 'line_numbers',
  ...EXPECTED_ERROR_TAG_TYPES,
}

// Regular expressions for breaking apart labels, contructing them this way makes it easier to only accept valid ones while only specifying things in one place.
const BLOCK_REG_EXP = Object.values(BLOCK_TYPES).join('|')
const TAG_REG_EXP = Object.values(STATIC_TAG_TYPES).concat(Object.values(EVAL_CONFIG_TAG_TYPES).map((cfg) => cfg.match)).join('|')
export const LABEL_REG_EXP = new RegExp(`^${LABEL_PREFIX}(${GROUP_NAME_REG_EXP}):(${BLOCK_REG_EXP})(:(?:(?:${TAG_REG_EXP}),)*(?:${TAG_REG_EXP})|)$`)

// CSS-style selector for finding the live code blocks in a document
export const BLOCK_SELECTOR = `code[data-lang^="${LABEL_PREFIX}"]`

// The name for the module file that gets evaluated (specifically shows up in error messages.)
export const EVAL_MODULE_NAME = 'module.rego'

// --- PREPROCESSING ---
// The maximum number of files to be processing at any given time. 0 indicates unlimited.
export const MAX_CONCUR_FILES = 1
// The maximum number of evaluations to be performing for a single processing file at any given time (net maximum is MAX_CONCUR_FILES * MAX_CONCUR_FILE_EVALS). 0 indicates unlimited.
export const MAX_CONCUR_FILE_EVALS = 50

// The version string for edge (e.g. not 'x.x.x[-x]')
export const VERSION_EDGE = 'edge'
export const VERSION_LATEST = 'latest'

// Platforms that there are OPA releases for.
export const PLATFORMS = {
  WINDOWS: 'windows',
  DARWIN: 'darwin',
  LINUX: 'linux'
}

// The folder where OPA binaries will be downloaded into.
// NOTE: Update package.json with any changes
export const OPA_CACHE_PATH = './opa_versions/'
export const OPA_EDGE_PATH = '../../../../'  // Repo root

// The amount of time between redownloads of the edge release (ms).
export const OPA_EDGE_CACHE_PERIOD = 24 * 60 * 60 * 1000 // 1 day

// The paths that the bundles will be accessible from.
// NOTE: Update package.json and inject.sh with any changes.
import pkg from '../package.json'
export const JS_BUNDLE_PATH = pkg.browser.replace(/dist/, '/js')
export const CSS_BUNDLE_BATH = JS_BUNDLE_PATH.replace(/js/g, 'css')

// --- UI ---
// Blocks on pages that match this regex will be non interactive (for limiting live functionality to the version that the playground supports).
export const INTERACTIVE_PATH = /^\/docs\/(latest|edge)/

// The path to initially open a new tab to with when opening a group in the playground
export const OPENING_IN_PLAYGROUND_PATH = '/live-blocks/opening-in-playground'

// How many blocks to hydrate before re-sorting the queue. Lower numbers ensure quick hydration if the page is scrolled while higher numbers make the entire process potentially faster.
export const HYDRATION_QUEUE_SORT_PERIOD = 5

// CodeMirror options
export const BASE_EDITOR_OPTS = {
  keyMap: 'styra',
}

export const READ_ONLY_EDITOR_OPTS = {
  readOnly: true,
  cursorBlinkRate: -1 // Hides cursor w/o readOnly = 'nocursor' so that editor can be focused and copy/pasting still works
}

export const LINE_NUMBERS_EDITOR_OPTS = {
  lineNumbers: true,
}

const REGO_MODE = 'rego'
const JSON_MODE = {name: 'javascript', json: true}
export const EDITOR_MODES = {
  [BLOCK_TYPES.MODULE]: REGO_MODE,
  [BLOCK_TYPES.QUERY]: REGO_MODE,
  [BLOCK_TYPES.INPUT]: JSON_MODE,
  [BLOCK_TYPES.OUTPUT]: JSON_MODE // JSON mode also works pretty well for pretty-formatted output tables
}

// Origin/ URL base for playground queries.
export const PLAYGROUND = 'https://play.openpolicyagent.org'

// --- ICONS ---
// NOTE: Update inject.sh and icons/ with any changes.
const icon = (title, src, spinning) => ({title, src, spinning})
const iconSRC = (name) => `/live-blocks/icons/${name}.svg`
const pgIconSRC = (name) => iconSRC(`playground-${name}`)
export const ICONS = {
  [BLOCK_TYPES.MODULE]: icon('Module', iconSRC('module')),
  [BLOCK_TYPES.QUERY]: icon('Query', iconSRC('query')),
  [BLOCK_TYPES.INPUT]: icon('Input', iconSRC('input')),
  [BLOCK_TYPES.OUTPUT]: icon('Output', iconSRC('output')),
  RESTORE: icon('Restore', iconSRC('restore')),
  PLAYGROUND: {
    READY: icon('Open in Playground', pgIconSRC('open')),
    WORKING: icon('Working...', pgIconSRC('working'), true),
    ERROR: icon('Failed!', pgIconSRC('error')),
  }
}

// --- ERRORS ---
// Predicates for each of the types of expected errors to determine whether the tag matches an OPA error object.
// Error objects from OPA have a `code` and typically have some `message`. For undefined decisions, the error is the value undefined.
const fIfErr = (pred) => (err) => { // Return false if an error occurs in the predicate (e.g. a field is not defined)
  try {
    return pred(err)
  } catch (e) {
    return false
  }
}
const COMPILE_ERR_CODE = 'rego_compile_error'
export const EXPECTED_ERROR_PREDICATES = {
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_ERROR]: () => true,
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_UNDEFINED]: (err) => err === undefined,
  // from 'ast'
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_REGO_ERROR]: fIfErr((err) => /rego_\w+_error/.test(err.code)),
  // [EXPECTED_ERROR_TAG_TYPES.EXPECT_PARSE_ERROR]: fIfErr((err) => err.code === 'rego_parse_error'), Not properly supported by the CLI.
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_COMPILE_ERROR]: fIfErr((err) => err.code === COMPILE_ERR_CODE),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_REGO_TYPE_ERROR]: fIfErr((err) => err.code === 'rego_type_error'),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_UNSAFE_VAR]: fIfErr((err) => err.code === 'rego_unsafe_var_error'),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_RECURSION]: fIfErr((err) => err.code === 'rego_recursion_error'),
  // from 'topdown'
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_EVAL_ERROR]: fIfErr((err) => /eval_\w+_error/.test(err.code)),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_CONFLICT]: fIfErr((err) => err.code === 'eval_conflict_error'),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_EVAL_TYPE_ERROR]: fIfErr((err) => err.code === 'eval_type_error'),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_BUILTIN_ERROR]: fIfErr((err) => err.code === 'eval_builtin_error'),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_WITH_MERGE_ERROR]: fIfErr((err) => err.code === 'eval_with_merge_error'),
  // more specific
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_ASSIGNED_ABOVE]: fIfErr((err) => err.code === COMPILE_ERR_CODE && /assigned above/.test(err.message)),
  [EXPECTED_ERROR_TAG_TYPES.EXPECT_REFERENCED_ABOVE]: fIfErr((err) => err.code === COMPILE_ERR_CODE && /referenced above/.test(err.message)),
}
