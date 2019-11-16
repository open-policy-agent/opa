import {ChainedError, OPAErrors} from '../errors'
import {EVAL_MODULE_NAME, PLAYGROUND} from '../constants'
import {getGroupData} from '../helpers'

// Evaluate the group using the playground. Either returns a string suitable for a code block or throws an error whose message is user-friendly. If the OPA evaluation itself failed, throws an OPAErrors instance.
export async function playgroundEval(groups, groupName) {
  // Get fetch options
  const opts = createDataRequestOpts(groups, groupName)

  // Make request
  let resp
  try {
    resp = await fetch(`${PLAYGROUND}/v1/data`, opts)
  } catch (e) {
    throw new ChainedError("can't contact server", e)
  }

  // Parse response and return
  let parsed
  try {
    parsed = await resp.json()
  } catch (e) {
    throw new ChainedError('invalid response from server', e)
  }

  if (resp.status === 200) {
    if (parsed.pretty) {
      return parsed.pretty.replace(/\n$/, '') // Some responses may have trailing newlines
    }
    if (parsed.result === null) {
      throw new OPAErrors("undefined decision", undefined)
    }
    // Else throw below
  } else if (parsed.message) { // The server returned a non-200 code and a message
    const message = parsed.message.replace(/\n$/, '') // Some messages may have trailing newlines
    if (parsed.error || message === 'undefined decision') { // The error was related to evaluation
      let opaErrors
      try { // Try validating the errors by creating an OPAErrors object to throw
        opaErrors = new OPAErrors(message, parsed.error)
      } catch (e) {
        throw new ChainedError(message, e) // Errors seem to be invalid but use the message it sent anyway, it's more informative than anything else.
      }
      throw opaErrors
    } else {
      throw new Error(message)
    }
  }

  // None of the return or throw cases matched, that's unexpected
  throw new Error('invalid response from server')
}

// Either resolves to a playground url with the group's content or rejects with a (non-user-friendly) error.
export async function shareToPlayground(groups, groupName) {
  let url = (await (await fetch(`${PLAYGROUND}/v1/share`, createDataRequestOpts(groups, groupName))).json()).result
  if (!url) {
    throw new Error("Unable to get URL from the playground's response.")
  }
  return url
}

// Constructs a the fetch options for querying the playground with a DataRequest. Throws an error with a user-friendly message if it can't.
function createDataRequestOpts(groups, groupName) {
  const {module, package: pkg, query, input, included} = getGroupData(groups, groupName) // Throws user-friendly messages.

  try {
    const body = {} // In the format accepted by playground backend
    body.rego_modules = {[EVAL_MODULE_NAME]: module, ...included}
    body.query_package = pkg
    if (query) {
      body.rego = query
    }
    if (input) {
      body.input = input
    }

    return {
      body: JSON.stringify(body),
      // Both types of requests are POSTing JSON
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    }
  } catch (e) { // Presumably stringifying the body failed, that shouldn't happen
    throw new ChainedError('unexpected error', e)
  }
}