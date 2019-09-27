import {EXPECTED_ERROR_PREDICATES} from './constants'
import {expectedErrorTags, toArray} from './helpers'

// An error caused by another one. Allows clearer or user-friendly messages while preserving stack information.
export class ChainedError extends Error {
  constructor(message, cause) {
    super(message)
    this.stack += `\n(caused by) ${cause ? ('stack' in cause ? cause.stack : cause) : 'no cause provided!'}`
  }
}

// Errors returned from an OPA evaluation call.
// Error objects from OPA have a `code` and typically have some `message`. For undefined decisions, the error is the value undefined.
export class OPAErrors extends Error {
  // Consumes the pretty-formatted message and the one or more (the latter as an array) errors that OPA produced.
  // In the case of undefined decisions, call this with 'undefined decision', undefined.
  // The constructor may error with a non-user-friendly message.
  constructor(pretty, errors) {
    super(pretty)
    this.errors = toArray(errors)

    // Validate there are more than one properly formatted errors
    if (!this.errors.length) {
      throw new Error('an OPAErrors object must contain at least one error')
    }
    for (let error of this.errors) {
      if (error != undefined && !('code' in error)) {
        throw new Error('an opa error should either be the value undefined (indicating an undefined decision) or have a code')
      }
    }
  }

  // Determine whether the given tag array corresponds to this' errors.
  matchesExpected(tags) {
    const eTags = expectedErrorTags(tags)

    return (
      // Check that all errors are matched by a tag
      this.errors.reduce((allErrsMatched, err) => allErrsMatched && (
        eTags.reduce((anyTagMatch, tag) => anyTagMatch || EXPECTED_ERROR_PREDICATES[tag](err), false)
      ), true)
      &&
      // Check that all tags match an error
      eTags.reduce((allTagsMatch, tag) => allTagsMatch && (
        this.errors.reduce((anyErrMatched, err) => anyErrMatched || EXPECTED_ERROR_PREDICATES[tag](err), false)
      ), true)
    )
  }
}