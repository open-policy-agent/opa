# directory-package-mismatch

**Summary**: Directory structure should mirror package

**Category**: Idiomatic

**Automatically fixable**: Yes

## Rationale

Quickly finding the package you're looking for in a policy repository is made much easier when package paths are
mirrored in the directory structure of your project. Meaning that if the name of your package is
`permissions.users.claims`, it should reside in a file under the `permissions/users/claims` directory. Note however
that any number of files can contribute to the same package! The `permissions/users/claims` directory may thus
contain several policy files that all declare `package permissions.users.claims`.

### Example

An example of directory structure for a project following this convention might look like this:

```shell
# Directory structure                   # Package path
.
├── README.md
└── bundle
    └── authorization
        ├── main.rego                   # authorization
        └── rbac
            ├── data.json               # authorization.rbac
            ├── roles
            │   └── roles.rego          # authorization.rbac.roles
            │   └── roles_test.rego     # authorization.rbac.roles_test
            └── users
                ├── customers.rego      # authorization.rbac.users
                ├── customers_test.rego # authorization.rbac.users_test
                ├── internal.rego       # authorization.rbac.users
                └── internal_test.rego  # authorization.rbac.users_test
```

### Tests

Astute observers may notice that the test files in the example above are placed in the same directory as the
policies they test. This may seem to contradict the
[test-outside-test-package](https://openpolicyagent.org/projects/regal/rules/testing/test-outside-test-package) rule, which
says that any test package should have a `_test` suffix in its package path. However, putting tests next to
the file they target arguably makes it _easier_ to find, and is a common practice in the OPA community. This
rule therefore by default ignores the `_test` suffix when determining whether the package path matches the
directory structure.

This behavior can be changed by setting the `exclude-test-suffix` configuration option to `false`, in which
case package paths with a `_test` suffix also will be required to reside in a directory with a `_test` suffix.

Setting the `exclude-test-suffix` option to `false` means the example from above would now look like this:

```shell
# Directory structure                   # Package path
.
├── README.md
└── bundle
    └── authorization
        ├── main.rego                   # authorization
        └── rbac
            ├── data.json               # authorization.rbac
            ├── roles
            │   └── roles.rego          # authorization.rbac.roles
            ├── roles_test
            │   └── roles_test.rego     # authorization.rbac.roles_test
            ├── users
            │   ├── customers.rego      # authorization.rbac.users
            │   └── internal.rego       # authorization.rbac.users
            └── users_test
                ├── customers_test.rego # authorization.rbac.users_test
                └── internal_test.rego  # authorization.rbac.users_test
```

Whichever way you choose is up to you. Consistency is key!

### Bundles

While directory structure doesn't matter to OPA when parsing _policies_, directories parsed as
[bundles](https://www.openpolicyagent.org/docs/management-bundles/) will read _data_ (`data.json` or
`data.yaml`) files and insert the data in the `data` document tree based on the directory structure relative
to the bundle root. Having policies structured in the same manner provides a uniform experience, and makes it
easier to understand where both policies and data come from.

### `regal fix` & Editor Support

Regal's [`fix` command](https://openpolicyagent.org/projects/regal/fixing) can automatically
rename files in a project to ensure compliance with this rule. This is
particularly useful when refactoring a project with many files.

:::info
Note that files will be renamed relative to their nearest root, see the
[documentation on roots](https://openpolicyagent.org/projects/regal#project-roots) when using
this rule with policy roots different from the project root.
:::

Editors integrating Regal's [language server](https://openpolicyagent.org/projects/regal/language-server) will automatically display
suggestions for idiomatic package paths based on the directory structure in which a policy resides. The image below
demonstrates a new policy being created inside an `authorization/rbac/roles` directory, and the editor
([via Regal](https://openpolicyagent.org/projects/regal/language-server#code-completions)) suggesting the package path
`authorization.rbac.roles`.

<img
src={require('../../assets/rules/pkg_name_completion.png').default}
alt="Package path auto-completion in VS Code"/>

In addition, empty files will be be 'formatted' to have the correct package
based on the directory structure. Newly created Rego files are treated in much
the same way. When a new file is created, the server will send a series of edits
back to set the content. If `exclude-test-suffix` is set to `false`, the file
will also be moved if required to the `_test` directory for that package.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    directory-package-mismatch:
      # one of "error", "warning", "ignore"
      level: error
      # exclude _test suffixes from package paths before comparing
      # them to directory structure paths. when set to false, a
      # package like authz.policy_test would need to be placed in
      # an authz/policy_test directory, and if set to true (default)
      # would be expected to be in authz/policy
      exclude-test-suffix: true
```

## Related Resources

- Rego Style Guide: [Package name should match file location](https://github.com/StyraInc/rego-style-guide#package-name-should-match-file-location)
- Regal Docs: [test-outside-test-package](https://openpolicyagent.org/projects/regal/rules/testing/test-outside-test-package)
- OPA Docs: [Bundles](https://www.openpolicyagent.org/docs/management-bundles/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/directory-package-mismatch/directory_package_mismatch.rego)
