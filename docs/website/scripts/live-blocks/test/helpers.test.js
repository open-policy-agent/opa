import chai, {expect} from 'chai'
import chaiAsPromised from 'chai-as-promised'
import sinon from 'sinon'
import {describe, it} from 'mocha'

import {batchMapFold, batchProcess, delay, expectedErrorTags, getAllGroupModules, getGroupData, getGroupField, handleLater, includedGroupNames, infoFromLabel, toArray} from '../src/helpers'
import {BLOCK_TYPES, STATIC_TAG_TYPES} from '../src/constants'

chai.use(chaiAsPromised)

describe('infoFromLabel', () => {
  it('parses a label', () => {
    const res = infoFromLabel('live:eg/pi:output:merge_down,expect_undefined,include(foobar/barfoo)')
    expect(res).has.keys(['group', 'type', 'tags'])
    expect(res.group).equals('eg/pi')
    expect(res.type).equals(BLOCK_TYPES.OUTPUT)
    expect(res.tags).lengthOf(3).and.contains('merge_down').and.contains('expect_undefined').and.contains('include(foobar/barfoo)')
  })

  it('parses a label without tags', () => {
    const res = infoFromLabel('live:eg/pi:output')
    expect(res).has.keys(['group', 'type', 'tags'])
    expect(res.group).equals('eg/pi')
    expect(res.type).equals('output')
    expect(res.tags).lengthOf(0)
  })

  it('errors on non-live block labels', () => {
    expect(() => infoFromLabel('json')).to.throw()
  })

  it('errors on malformed block labels', () => {
    expect(() => infoFromLabel('live:eg:modle')).to.throw()
    expect(() => infoFromLabel('live::module')).to.throw()
    expect(() => infoFromLabel('live:eg:module:')).to.throw()
    expect(() => infoFromLabel('live:eg:module:foobar')).to.throw()
    expect(() => infoFromLabel('live:eg:module:expect_undefined')).to.throw()
    expect(() => infoFromLabel('live:eg:module:include(foobar/barfoo)')).to.throw()
  })
})

describe('expectedErrorTags', () => {
  it('accepts an empty array', () => {
    expect(expectedErrorTags([])).deep.equals([])
  })

  it('can consume only non-expectations', () => {
    expect(expectedErrorTags([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.MERGE_DOWN, 'include(foobar)'])).deep.equals([])
  })

  it('can consume only expectations', () => {
    expect(expectedErrorTags([STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.EXPECT_ERROR]))
      .has.lengthOf(2)
      .and.contains(STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR)
      .and.contains(STATIC_TAG_TYPES.EXPECT_ERROR)
  })

  it('can consume a mixture', () => {
    expect(expectedErrorTags([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.MERGE_DOWN, STATIC_TAG_TYPES.EXPECT_ERROR]))
      .has.lengthOf(2)
      .and.contains(STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR)
      .and.contains(STATIC_TAG_TYPES.EXPECT_ERROR)
  })

  it('doesn\'t mutate argument', () => {
    const tags = [STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.MERGE_DOWN, STATIC_TAG_TYPES.EXPECT_ERROR]
    const args = tags.slice()

    expectedErrorTags([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.MERGE_DOWN, STATIC_TAG_TYPES.EXPECT_ERROR])
    expect(args).to.deep.equal(tags)
  })
})

describe('includedGroupNames', () => {
  it('accepts an empty array', () => {
    expect(includedGroupNames([])).deep.equals([])
  })

  it('can consume only non-includes', () => {
    expect(includedGroupNames([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.MERGE_DOWN, STATIC_TAG_TYPES.EXPECT_ASSIGNED_ABOVE])).deep.equals([])
  })

  it('can consume only includes', () => {
    expect(includedGroupNames(['include(foo/bar)', 'include(eg)']))
      .has.lengthOf(2)
      .and.contains('foo/bar')
      .and.contains('eg')
  })

  it('can consume a mixture', () => {
    expect(includedGroupNames([STATIC_TAG_TYPES.HIDDEN, 'include(foo/bar)', STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.MERGE_DOWN, 'include(eg)', STATIC_TAG_TYPES.EXPECT_ERROR]))
      .has.lengthOf(2)
      .and.contains('foo/bar')
      .and.contains('eg')
  })

  it('doesn\'t mutate argument', () => {
    const tags = [STATIC_TAG_TYPES.HIDDEN, 'include(foo/bar)', STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.MERGE_DOWN, 'include(eg)', STATIC_TAG_TYPES.EXPECT_ERROR]
    const args = tags.slice()

    includedGroupNames([STATIC_TAG_TYPES.HIDDEN, STATIC_TAG_TYPES.EXPECT_BUILTIN_ERROR, STATIC_TAG_TYPES.MERGE_DOWN, STATIC_TAG_TYPES.EXPECT_ERROR])
    expect(args).to.deep.equal(tags)
  })

  it('only accepts properly formated tags', () => {
    expect(includedGroupNames(['include(foo-bar)', 'include', 'include()', 'include(foo/bar/)']))
      .has.lengthOf(0)
  })
})

describe('getGroupField', () => {
  it('gets field in group if there', () => {
    expect(getGroupField({foo: {bar: 1}, 'foo/two': {bar: 2}}, 'foo/two', 'bar')).to.equal(2)
  })

  it('gets parents\' fields when field is missing', () => {
    expect(getGroupField({foo: {bar: 7}, 'foo/two': {baz: 42}}, 'foo/two', 'bar')).to.equal(7)
  })

  it('gets parents\' fields when group is missing', () => {
    expect(getGroupField({foo: {bar: 7}}, 'foo/two', 'bar')).to.equal(7)
  })

  it('gets field from deepest parent', () => {
    const groups = {foo: {bar: 1}, 'foo/two': {bar: 2}, 'foo/two/child': {}}
    expect(getGroupField(groups, 'foo/two/child', 'bar')).to.equal(2)
  })

  it('skips missing parents', () => {
    expect(getGroupField({foo: {bar: 7}, 'foo/two/three': {baz: 42}}, 'foo/two/three', 'bar')).to.equal(7)
  })

  it('returns undefined when hierarchy does not exist', () => {
    expect(getGroupField({foo: {bar: 7}, 'foo/two/three': {baz: 42}}, 'a/b/c', 'bar')).to.equal(undefined)
  })

  it('returns undefined when out of parents', () => {
    expect(getGroupField({foo: {bar: 7}, 'foo/two': {baz: 42}}, 'foo/two', 'unset')).to.equal(undefined)
  })

  it('doesn\'t mutate groups', () => {
    const groups = {foo: {bar: 7}, 'foo/two': {baz: 42}}
    const arg = JSON.parse(JSON.stringify(groups))
    getGroupField(arg, 'foo/two', 'bar')
    expect(arg).to.deep.equal(groups)
    getGroupField(arg, 'foo/two', 'unset')
    expect(arg).to.deep.equal(groups)
  })
})

describe('getAllGroupModules', () => {
  it('can get no modules', () => {
    expect(getAllGroupModules({}, 'foo')).to.deep.equal([])
    expect(getAllGroupModules({foo: {bar: 7}, 'foo/two': {baz: 42}}, 'foo/two')).to.deep.equal([])
  })

  it('gets all the present modules from shallowest to deepest', () => {
    expect(getAllGroupModules({
      'a': {module: 1},
      'a/b/c': {module: 2}
    }, 'a/b/c/d')).to.deep.equal([1, 2])
  })

  it('doesn\'t mutate groups', () => {
    const groups = {
      'a': {module: 1},
      'a/b/c': {module: 2}
    }
    const arg = JSON.parse(JSON.stringify(groups))
    getAllGroupModules(arg, 'a/b/c/d')
    expect(arg).to.deep.equal(groups)
  })
})

describe('getGroupData', () => {
  it('concatenates module blocks', () => {
    expect(getGroupData({
      'a': {module: {get: () => 'package foo\na'}},
      'a/b/c': {module: {get: () => 'c'}}
    }, 'a/b/c/d').module).to.equal('package foo\na\n\nc')
  })

  it('errors when there are no module blocks', () => {
    expect(() => getGroupData({
      'a': {query: {get: () => 'a'}},
      'a/b/c': {query: {get: () => 'c'}}
    }, 'a/b/c/d')).to.throw('no module')
  })

  it('get\'s the module package', () => {
    expect(getGroupData({
      'a': {module: {get: () => 'package foo\na'}},
      'a/b/c': {module: {get: () => 'c'}}
    }, 'a/b/c/d').package).to.equal('foo')
  })

  it('errors when there is no package declaration', () => {
    expect(() => getGroupData({
      'a': {module: {get: () => 'a'}},
      'a/b/c': {module: {get: () => 'c'}}
    }, 'a/b/c/d')).to.throw(/package/)
  })

  const egInput = ['foo', {bar: 'baz'}]
  const strInput = JSON.stringify(egInput)

  it('parses input from block', () => {
    expect(getGroupData({
      'a': {input: {get: () => 'foobar'}, module: {get: () => 'package foo\na'}}, // Will error if gets from this parent
      'a/b/c': {input: {get: () => strInput}, module: {get: () => 'c'}}
    }, 'a/b/c').input).to.deep.equal(egInput)
  })

  it('can get input from parents', () => {
    expect(getGroupData({
      'a': {input: {get: () => 'foobar'}, module: {get: () => 'package foo\na'}}, // Will error if gets from this parent
      'a/b/c': {input: {get: () => strInput}, module: {get: () => 'c'}}
    }, 'a/b/c/d').input).to.deep.equal(egInput)
  })

  it('errors when input cannot be parsed', () => {
    expect(() => getGroupData({
      'a': {input: {get: () => 'foobar'}, module: {get: () => 'package foo\na'}},
    }, 'a')).to.throw('can\'t parse input')
  })

  it('gets the query', () => {
    expect(getGroupData({
      'a': {query: {get: () => {
        throw new Error()
      }}, module: {get: () => 'package foo\na'}}, // Will error if gets from this parent
      'a/b/c': {query: {get: () => 'foobar'}, module: {get: () => 'c'}}
    }, 'a/b/c').query).to.equal('foobar')
  })

  it('can get the query from parents', () => {
    expect(getGroupData({
      'a': {query: {get: () => {
        throw new Error()
      }}, module: {get: () => 'package foo\na'}}, // Will error if gets from this parent
      'a/b/c': {query: {get: () => 'foobar'}, module: {get: () => 'c'}}
    }, 'a/b/c/d').query).to.equal('foobar')
  })

  it('includes other modules', () => {
    expect(getGroupData({
      'a': {module: {get: () => 'package foo\na'}},
      'a/b/c': {module: {get: () => 'c'}, output: {tags: ['include(d)']}},
      'd': {module: {get: () => 'd'}}
    }, 'a/b/c').included["d.rego"]).to.equal('d')
  })
})

describe('handleLater', () => {
  it('resolves with the resolved value', async () => {
    expect(await handleLater(() => Promise.resolve('foo'))).to.deep.equal(['foo', undefined])
  })

  it('resolves with the rejected value', async () => {
    expect(await handleLater(() => Promise.reject('foo'))).to.deep.equal([undefined, 'foo'])
  })

  it('resolves with resolved undefined', async () => {
    expect(await handleLater(() => Promise.resolve(undefined))).to.deep.equal([undefined, undefined])
  })

  it('resolves with rejected undefined', async () => {
    expect(await handleLater(() => Promise.reject(undefined))).to.deep.equal([undefined, undefined])
  })
})

describe('batchProcess', () => {
  it('can run no jobs', async () => {
    expect(await batchProcess(0, [])).to.deep.equal([])
  })

  it('runs only maxC jobs at a time', async () => {
    expect(await time(() =>
      batchProcess(1, [
        async () => await delay(100),
        async () => await delay(100),
      ])
    )).to.equal(200)

    expect(await time(() =>
      batchProcess(2, [
        async () => await delay(100),
        async () => await delay(100),
        async () => await delay(100),
        async () => await delay(100),
      ])
    )).to.equal(200)
  })

  it('runs all of them simultaneously with maxC = 0', async () => {
    expect(await time(() =>
      batchProcess(0, [
        async () => await delay(100),
        async () => await delay(100),
      ])
    )).to.equal(100)
  })

  it('resolves to an array of the results', async () => {
    expect(await batchProcess(0, [() => 1, () => 2])).to.have.a.lengthOf(2).and.contain(1).and.contain(2)
  })

  it('rejects once all are run with a flat array of errors', async () => {
    const [_, e] = await handleLater(() => batchProcess(0, [() => {
      throw [1, 2]
    }, () => {
      throw 3
    }]))
    expect(e).to.have.a.lengthOf(3).and.contain(1).and.contain(2).and.contain(3)
  })

  it('doesn\'t mutate jobs', async () => {
    const jobs = [() => 1, () => 2]
    const arg = jobs.slice()
    await batchProcess(0, arg)
    expect(arg).to.deep.equal(jobs)
  })
})

describe('batchMapFold', () => {
  it('can handle no input', async () => {
    expect(await batchMapFold(0, [], () => {
      throw ''
    }, () => {
      throw ''
    }, 'foo')).to.equal('foo')
  })

  it('handles only maxC inputs at a time', async () => {
    expect(await time(() =>
      batchMapFold(1, [0, 0], () => delay(100), () => {}, 0)
    )).to.equal(200)

    expect(await time(() =>
      batchMapFold(2, [0, 0, 0, 0], () => delay(100), () => {}, 0)
    )).to.equal(200)
  })

  it('handles all of them simultaneously with maxC = 0', async () => {
    expect(await time(() =>
      batchMapFold(0, [0, 0, 0, 0], () => delay(100), () => {}, 0)
    )).to.equal(100)
  })

  it('resolves to the folded results', async () => {
    expect(await batchMapFold(0, [1, 2, 4, 8], (n) => n, (o, {output: [n]}) => o + n, 0)).to.equal(15)
  })

  it('reports map errors to the folder', async () => {
    expect(await batchMapFold(0, [1, 2, 4, 8], (n) => {
      throw n
    }, (o, {output: [, n]}) => o + n, 0)).to.equal(15)
  })

  it('folds synchronously', async () => {
    expect(await batchMapFold(0, [1, 2, 4, 8], (n) => n, async (o, {output: [n]}) => o + n, 0)).to.equal('[object Promise]1')
  })

  it('doesn\'t mutate inputs', async () => {
    const inputs = [1, 2, 4, 8]
    const arg = inputs.slice()
    await batchMapFold(0, arg, (n) => n, (o, {output: [n]}) => o + n, 0)
    expect(arg).to.deep.equal(inputs)
  })

  it('can mutate the out argument', async () => {
    const out = {val: 0}
    await batchMapFold(0, [1, 2, 4, 8], (n) => n, (o, {output: [n]}) => {
      o.val += n; return o
    }, out)
    expect(out.val).to.equal(15)
  })
})

describe('delay', () => {
  it('delays the given amount', async () => {
    expect(await time(() => delay(100))).to.equal(100)
  })
})

describe('toArray', () => {
  it('returns arrays', () => {
    const empty = []
    expect(toArray(empty)).to.equal(empty) // Not just deep; actually equal
    const full = [1, 2, 3]
    expect(toArray(full)).to.equal(full)
  })

  it('wraps non-arrays', () => {
    expect(toArray(undefined)).to.deep.equal([undefined])
    expect(toArray('a')).to.deep.equal(['a'])
    expect(toArray({a: 'a'})).to.deep.equal([{a: 'a'}])
  })
})

// Time how long it takes an async function to complete without actually waiting that long. Returns a promise that resolves to the # of ms.
async function time(f) {
  const clock = sinon.useFakeTimers() // Will only advance automatically if runAll() doesn't work.

  // Run the function
  let done = false;
  (async () => {
    await handleLater(f) // await f to either resolve or reject
    done = true
  })()

  // Run the clock, allowing the function's delays to complete
  while (!done) {
    clock.runAll() // Even though it's called 'runAll', calling it multiple times is necessary
    await Promise.resolve() // Yield to the function that's running
  }

  // Finish up
  const end = new Date().getTime()
  clock.restore()
  return end
}