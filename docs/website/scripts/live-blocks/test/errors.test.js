import {describe, it} from 'mocha'
import {expect} from 'chai'

import {ChainedError, OPAErrors} from '../src/errors'
import {STATIC_TAG_TYPES} from '../src/constants'

describe('ChainedError', () => {
  it('uses a new message', () => {
    expect((new ChainedError('b', new Error('a'))).message).to.equal('b')
  })

  it('preserves old stack information', () => {
    const e = new Error()
    e.stack = 'foobar'
    expect((new ChainedError('', e)).stack).to.contain('foobar')
  })

  it('won\'t throw an unintelligible error if you forget the cause', () => {
    expect((new ChainedError('')).stack).to.contain('no cause')
  })
})

describe('OPAErrors', () => {
  const oe = (code) => ({code})

  it('message is the given pretty string', () => {
    expect((new OPAErrors('foobar', oe(''))).message).to.equal('foobar')
  })

  it('can be constructed with a single OPA error', () => {
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([STATIC_TAG_TYPES.EXPECT_RECURSION])).to.be.true
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR])).to.be.false
  })

  it('can be constructed with an array of OPA errors', () => {
    expect((new OPAErrors('', [oe('rego_recursion_error'), oe('rego_compile_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_RECURSION, STATIC_TAG_TYPES.EXPECT_COMPILE_ERROR])).to.be.true
    expect((new OPAErrors('', [oe('rego_recursion_error'), oe('rego_compile_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_RECURSION, STATIC_TAG_TYPES.EXPECT_UNSAFE_VAR])).to.be.false
  })

  it('can be constructed with undefined as the error', () => {
    expect((new OPAErrors('', undefined)).matchesExpected([STATIC_TAG_TYPES.EXPECT_UNDEFINED])).to.be.true
    expect((new OPAErrors('', undefined)).matchesExpected([STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR])).to.be.false

    expect((new OPAErrors('', [undefined])).matchesExpected([STATIC_TAG_TYPES.EXPECT_UNDEFINED])).to.be.true
    expect((new OPAErrors('', [undefined])).matchesExpected([STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR])).to.be.false
  })

  it('can\'t be constructed with improperly formatted OPA errors', () => {
    expect(() => new OPAErrors('', 'foobar')).to.throw()
    expect(() => new OPAErrors('', {message: 'foobar'})).to.throw()
    expect(() => new OPAErrors('', [oe('rego_recursion_error'), {message: 'foobar'}])).to.throw()
  })

  it('can\'t be constructed with empty array of OPA errors', () => {
    expect(() => new OPAErrors('', [])).to.throw()
  })

  it('can match against no tags', () => {
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([])).to.be.false
  })

  it('can match with duplicate errors', () => {
    expect((new OPAErrors('', [oe('rego_recursion_error'), oe('rego_recursion_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_RECURSION])).to.be.true
  })

  it('can match with duplicate tags', () => {
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([STATIC_TAG_TYPES.EXPECT_RECURSION, STATIC_TAG_TYPES.EXPECT_RECURSION])).to.be.true
  })

  it('can match with non-error tags', () => {
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.EXPECT_RECURSION])).to.be.true
  })

  it('requires all errors to be matched', () => {
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([STATIC_TAG_TYPES.HIDDEN])).to.be.false
    expect((new OPAErrors('', [oe('rego_recursion_error'), oe('rego_compile_error')])).matchesExpected([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.EXPECT_RECURSION])).to.be.false
  })

  it('requires all error tags to match an error', () => {
    expect((new OPAErrors('', oe('rego_recursion_error'))).matchesExpected([STATIC_TAG_TYPES.HIDDEN])).to.be.false
    expect((new OPAErrors('', [oe('rego_recursion_error'), oe('rego_compile_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_RECURSION, STATIC_TAG_TYPES.EXPECT_COMPILE_ERROR, STATIC_TAG_TYPES.EXPECT_ASSIGNED_ABOVE])).to.be.false
  })

  it('child errors can be matched precisely', () => {
    expect((new OPAErrors('', [{code: 'rego_compile_error', message: 'assigned above'}])).matchesExpected([STATIC_TAG_TYPES.EXPECT_ASSIGNED_ABOVE])).to.be.true
    expect((new OPAErrors('', [{code: 'rego_compile_error', message: 'assigned above'}, {code: 'rego_compile_error', message: 'referenced above'}])).matchesExpected([STATIC_TAG_TYPES.EXPECT_ASSIGNED_ABOVE])).to.be.false
  })

  it('parent tags match children', () => {
    expect((new OPAErrors('', [undefined, oe('rego_type_error'), oe('eval_type_error'), oe('fake_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_ERROR])).to.be.true
    expect((new OPAErrors('', [oe('rego_type_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_REGO_ERROR])).to.be.true
    expect((new OPAErrors('', [oe('eval_type_error')])).matchesExpected([STATIC_TAG_TYPES.EXPECT_EVAL_ERROR])).to.be.true
    expect((new OPAErrors('', [{code: 'rego_compile_error', message: 'assigned above'}])).matchesExpected([STATIC_TAG_TYPES.EXPECT_COMPILE_ERROR])).to.be.true
  })

  it('undefined is only matched by its own tag and expect_error', () => {
    expect((new OPAErrors('', undefined).matchesExpected([STATIC_TAG_TYPES.EXPECT_ERROR, STATIC_TAG_TYPES.EXPECT_UNDEFINED]))).to.be.true
    expect((new OPAErrors('', undefined).matchesExpected([STATIC_TAG_TYPES.EXPECT_EVAL_ERROR, STATIC_TAG_TYPES.EXPECT_UNDEFINED]))).to.be.false
  })
})