# Documentation and Website Development

This README contains an overview of the development environment for the
OPA website.

## Development

Those contributing to the OPA website must first install the Node packages to
run it:

```bash
make install
```

Once these have been installed, a live updating development server can be
started by running

```bash
make dev
```

Note, that the server will not restart when you change the
`docusaurus.config.js`.

You can run a local build of the website by running:

```bash
make build
```

This will build a version of the site ready for deployment. Broken links are
also checked in this step.
