## 1.0.0

### Miscellaneous

- Dependency updates; notably:
  - build(deps): Add example.com/newdep 0.1.0
  - build(deps): Bump example.com/stable from 1.1.0 to 1.2.0
  - build(deps): Drop example.com/legacy (was 0.4.0)
  - deps: Bump example.com/widget from 2.0.0 to 3.0.0 ([#1220](https://github.com/example/example/pull/1220)) authored by @carol
- Reduce allocations in the evaluator ([#1212](https://github.com/example/example/pull/1212)) authored by @erin
- Tidy up the README ([#1211](https://github.com/example/example/pull/1211)) authored by @dave
- ast,storage/inmem: Share interned values ([#1209](https://github.com/example/example/pull/1209)) authored by @bob
- ast: Reject duplicate imports ([#104](https://github.com/example/example/issues/104), [#105](https://github.com/example/example/issues/105)) authored by @erin, reported by @bob
- bundle: Add support for lazy manifest parsing ([#1203](https://github.com/example/example/pull/1203)) authored by @dave
- docs: Fix a broken link in the policy guide ([`0006000`](https://github.com/example/example/commit/0006000600060006000600060006000600060006)) authored by @alice
- download: Use oras instead of example.com/containerd ([#1221](https://github.com/example/example/pull/1221)) authored by @dave
- loader: Tolerate a trailing newline in .manifest ([#1214](https://github.com/example/example/pull/1214))
- runtime: Close the decision log writer on shutdown ([#106](https://github.com/example/example/issues/106)) authored by @frank, reported by @alice
- server: Reject requests with an invalid Content-Type ([#101](https://github.com/example/example/issues/101)) authored by @alice, reported by @bob
- topdown: Add a fast path for single-key objects ([#1210](https://github.com/example/example/pull/1210)) authored by @carol
- topdown: Fix panic in `object.union_n` with empty input ([#102](https://github.com/example/example/issues/102)) reported and authored by @carol
- topdown: `http.send` now honours the caching directive ([#1213](https://github.com/example/example/pull/1213)) authored by @frank
