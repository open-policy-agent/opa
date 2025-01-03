import cp from 'child_process'
import util from 'util'
import {promises as promFS} from 'fs'

import tmp from 'tmp'
import semver from 'semver'

import {ChainedError, OPAErrors} from '../errors'
import {EVAL_MODULE_NAME, VERSION_EDGE, VERSION_LATEST} from '../constants'
import {getGroupData} from '../helpers'

import {acquireOPAVersion} from './acquireOPAVersion'

// Clean up temporary files on exit
tmp.setGracefulCleanup()

// Promisify non-fs async file operations
const execFile = util.promisify(cp.execFile)
const tmpFile = util.promisify(tmp.file)

// Evaluate the group using a local OPA binary. Either returns a string suitable for a code block or throws an error whose message is user-friendly. If the OPA evaluation itself failed, throws an OPAErrors instance.
export default async function localEval(groups, groupName, opaVersion) {
  const opa = await acquireOPAVersion(opaVersion) // Get the path (and download if necessary) the required version of OPA.

  const [args, moduleFilenameMap] = await prepEval(groups, groupName, opaVersion) // Save the group data to temporary files

  try {
    return (await execFile(opa, args('pretty'))).stdout.replace(/\n$/, '') // Evaluate, retrieving the pretty-formatted output
  } catch (e) {
    if (e.stdout) { // OPA returned an error message
      let pretty = e.stdout.replace(/\n$/, '')
      // Replace the module file names with what the playground will use.
      for (let [tmp, replacement] of Object.entries(moduleFilenameMap)) {
        pretty = pretty.replace(new RegExp(tmp, 'g'), replacement)
      }

      if (pretty === 'undefined') { // Special undefined case, use the same message as the playground.
        throw new OPAErrors('undefined decision', undefined)

      } else { // Some other error occurred, reevaluate to get the JSON-formatted version of the errors so that they can be compared against what the block expects.
        try {
          await execFile(opa, args('json'))
          throw new Error('subsequent eval of failing evaluation did not fail')
        } catch (e2) {
          if (e2.stdout) { // Reevaluation seems to have worked
            let opaErrors
            try { // Parse the reevaluation result into an OPAErrors object to be thrown.
              let parsed = JSON.parse(e2.stdout)
              if (parsed.errors) {
                opaErrors = new OPAErrors(pretty, parsed.errors)
              } else {
                opaErrors = new OPAErrors(pretty, parsed.error)
              }
            } catch (e3) {
              throw new ChainedError('invalid response while trying to get details about an evaluation failure', e3)
            }
            throw opaErrors
          } else { // Reevaluation didn't produce output
            throw new ChainedError('an error occurred while trying to get details about an evaluation failure', e2)
          }
        }
      }
    } else { // OPA didn't return an error message, probably some problem calling it.
      throw new ChainedError(e.stderr ? `OPA: ${e.stderr}` : 'unable to perform local evaluation', e) // Some OPA error. This particularly applies to parse errors, they occur at a different point internally and are not reported correctly; it's fine though, doc examples probably shouldn't cause them.
    }
  }
}

// Returns 1st a function that consumes a string for the output format you want and produces an array of arguments and 2nd a map of module file names to strings that should be replaced in error messages. May throw a user-friendly error.
async function prepEval(groups, groupName, opaVersion) {
  const {module, package: pkg, query, input, included} = getGroupData(groups, groupName)
  const base = ['eval', '--fail'] // Fail on undefined
  const rest = []
  const moduleFilenameMap = {}

  try {
    let tmpModule = await tmpFile({postfix: '.rego'})
    await promFS.writeFile(tmpModule, module)
    rest.push('-d', tmpModule, '--package', pkg)
    moduleFilenameMap[tmpModule] = EVAL_MODULE_NAME

    if (input) {
      let stringified = ''
      try {
        stringified = JSON.stringify(input)
      } catch (e) {
        throw new ChainedError('input cannot be stringified', e)
      }
      const tmpInput = await tmpFile({postfix: '.json'})
      await promFS.writeFile(tmpInput, stringified)
      rest.push('--input', tmpInput)
    }

    if (query) {
      // NOTE(sr): Only do this if the version is recent enough to understand it,
      // and if we're rendering docs of a version where we actually started using
      // queries that need this import.
      if (opaVersion === VERSION_LATEST || opaVersion == VERSION_EDGE ||
        semver.satisfies(semver.coerce(opaVersion), '>=0.42.0')) {
        rest.push('--import', 'future.keywords') // queries may contain them
      }
      rest.push(query)
    } else { // Simulate playground default behavior
      if (included) {
        rest.push('data')
      } else {
        rest.push(`data.${pkg}`)
      }
    }

    if (included) {
      for (let [incName, incModule] of Object.entries(included)) {
        let tmpIncluded = await tmpFile({postfix: '.rego'})
        await promFS.writeFile(tmpIncluded, incModule)
        rest.push('-d', tmpIncluded)
        moduleFilenameMap[tmpIncluded] = incName
      }
    }

    return [(format) => [...base, `--format=${format}`, ...rest], moduleFilenameMap]
  } catch (e) {
    throw new ChainedError('a problem occurred while preparing to evaluate', e)
  }
}
