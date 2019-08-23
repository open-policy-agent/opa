import cp from 'child_process'
import util from 'util'
import {promises as promFS} from 'fs'

import tmp from 'tmp'

import {ChainedError, OPAErrors} from '../errors'
import {EVAL_MODULE_NAME} from '../constants'
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

  const [args, tmpModulePath] = await prepEval(groups, groupName) // Save the group data to temporary files

  try {
    return (await execFile(opa, args('pretty'))).stdout.replace(/\n$/, '') // Evaluate, retrieving the pretty-formatted output
  } catch (e) {
    if (e.stdout) { // OPA returned an error message
      const pretty = e.stdout.replace(/\n$/, '').replace(new RegExp(tmpModulePath, 'g'), EVAL_MODULE_NAME) // Replace the module file name with what the playground will use.

      if (pretty === 'undefined') { // Special undefined case, use the same message as the playground.
        throw new OPAErrors('undefined decision', undefined)

      } else { // Some other error occured, reevaluate to get the JSON-formatted version of the errors so that they can be compared against what the block expects.
        try {
          await execFile(opa, args('json'))
          throw new Error('subsequent eval of failing evaluation did not fail')
        } catch (e2) {
          if (e2.stdout) { // Reevaluation seems to have worked
            let opaErrors
            try { // Parse the reevaluation result into an OPAErrors object to be thrown.
              opaErrors = new OPAErrors(pretty, JSON.parse(e2.stdout).error)
            } catch (e3) {
              throw new ChainedError('invalid response while trying to get details about an evaluation failure', e3)
            }
            throw opaErrors
          } else { // Reevaluation didn't produce output
            throw new ChainedError('an error occured while trying to get details about an evaluation failure', e2)
          }
        }
      }
    } else { // OPA didn't return an error message, probably some problem calling it.
      throw new ChainedError(e.stderr ? `OPA: ${e.stderr}` : 'unable to perform local evaluation', e) // Some OPA error. This particularly applies to parse errors, they occur at a different point internally and are not reported correctly; it's fine though, doc examples probably shouldn't cause them.
    }
  }
}

// Returns 1st a function that consumes a string for the output format you want and produces an array of arguments and 2nd the name of the module file used in those arguments. May throw a user-friendly error.
async function prepEval(groups, groupName) {
  const {module, query, input} = getGroupData(groups, groupName)
  const base = ['eval', '--fail'] // Fail on undefined
  const rest = []

  let pkgRegexRes = /^package[ \t]+([\w.-]+)[ \t]*#*.*$/m.exec(module)
  if (!pkgRegexRes) {
    throw new Error('couldn\'t find package declaration in module')
  }
  try {
    const pkg = pkgRegexRes[1]
    let tmpModule = await tmpFile({postfix: '.rego'})
    await promFS.writeFile(tmpModule, module)
    rest.push('-d', tmpModule, '--package', pkg)

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
      rest.push(query)
    } else {
      rest.push(`data.${pkg}`)
    }

    return [(format) => [...base, `--format=${format}`, ...rest], tmpModule]
  } catch (e) {
    throw new ChainedError('a problem occured while preparing to evaluate', e)
  }
}
