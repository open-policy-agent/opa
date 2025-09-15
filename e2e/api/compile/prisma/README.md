# E2E Prisma helper

This is a tiny script that is used to run our E2E tests via Prisma and [@open-policy-agent/ucast-prisma](https://www.npmjs.com/package/@open-policy-agent/ucast-prisma).

It's invoked by `e2e/compile/e2e_test.go` for a subset of the tests.
It reads UCAST conditions (as JSON) on STDIN, converts them to Prisma filters, and then runs
a "SELECT * FROM ..." query (findMany) against the `fruits` table like the other tests.
The returned rows are printed as JSON on STDOUT.


## Test a change

To use a local change to ucast-prisma with the Compile E2E suite, you'll have to do this:

In `package.json`, replace the line with "@open-policy-agent/ucast-prisma" with the following:

```json
"@open-policy-agent/ucast-prisma": "file:../../../opa-typescript/packages/ucast-prisma"
```

Also run `npm run build` in "opa-typescript/packages/ucast-prisma" to make sure the changes are built.
Run `npm i` in `e2e/prisma`, and then run the E2E tests for the prisma-enabled subset:

```
go test -tags e2e ./compile -run 'TestCompileHappyPathE2E/postgres/prisma/.' -v
```
