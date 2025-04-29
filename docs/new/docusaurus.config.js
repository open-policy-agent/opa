const { themes } = require("prism-react-renderer");
const lightCodeTheme = themes.github;
const darkCodeTheme = themes.dracula;
const semver = require("semver");
import fs from "fs/promises";
const path = require("path");

const { loadPages, loadEcosystemPages } = require("./src/lib/ecosystem/loadPages");

// TODO: update this to "/" when this is the main site.
const baseUrl = "/new/";

// With JSDoc @type annotations, IDEs can provide config autocompletion
/** @type {import("@docusaurus/types").DocusaurusConfig} */
(
  module.exports = {
    title: "Open Policy Agent",
    tagline: "Policy-based control for cloud native environments",
    url: "https://openpolicyagent.org",
    baseUrl: baseUrl,
    // Build-time options
    onBrokenLinks: "throw",
    onBrokenMarkdownLinks: "throw",
    trailingSlash: false,
    presets: [
      [
        "@docusaurus/preset-classic",
        /** @type {import("@docusaurus/preset-classic").Options} */
        {
          docs: {
            path: "docs",
            routeBasePath: "/docs/",
            breadcrumbs: false,
            sidebarPath: require.resolve("./src/lib/sidebars.js"),
          },
          blog: false,
          theme: {
            customCss: require.resolve("./src/css/custom.css"),
          },
        },
      ],
    ],

    themeConfig: {
      colorMode: {
        disableSwitch: true,
        respectPrefersColorScheme: true,
      },
      metadata: [
        { name: "msapplication-TileColor", content: "#2b5797" },
        { name: "theme-color", content: "#ffffff" },
      ],
      headTags: [
        {
          tagName: "link",
          attributes: {
            rel: "icon",
            href: "/favicon.ico",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "apple-touch-icon",
            sizes: "180x180",
            href: "/apple-touch-icon.png",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "icon",
            type: "image/png",
            sizes: "32x32",
            href: "/favicon-32x32.png",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "icon",
            type: "image/png",
            sizes: "16x16",
            href: "/favicon-16x16.png",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "manifest",
            href: "/site.webmanifest",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "mask-icon",
            href: "/safari-pinned-tab.svg",
            color: "#5bbad5",
          },
        },
      ],
      navbar: {
        title: "Open Policy Agent",
        logo: {
          alt: "OPA Logo",
          src: "img/nav/logo.png",
        },
        items: [
          { to: "/docs/", label: "Docs", position: "right" },
          { to: "/ecosystem/", label: "Ecosystem", position: "right" },
          { to: "/security", label: "Security", position: "right" },
          { to: "/support", label: "Support", position: "right" },
          { to: "/community", label: "Community", position: "right" },
          { href: "https://play.openpolicyagent.org/", label: "Play", position: "right" },
          { href: "https://blog.openpolicyagent.org/", label: "Blog", position: "right" },
          {
            type: "html",
            position: "right",
            value: `
        <a href="https://github.com/open-policy-agent/opa"
           target="_blank"
           rel="noopener noreferrer"
           aria-label="GitHub repository">
          <img src="${baseUrl}img/nav/github.png" alt="GitHub" style="width: 24px; height: auto; margin-left: 8px;" />
        </a>
      `,
          },
          {
            type: "html",
            position: "right",
            value: `
        <a href="https://slack.openpolicyagent.org/"
           target="_blank"
           rel="noopener noreferrer"
           aria-label="Slack community">
          <img src="${baseUrl}img/nav/slack.png" alt="Slack" style="width: 24px; height: auto; margin-left: 8px;" />
        </a>
      `,
          },
        ],
      },
      footer: {
        style: "light",
        links: [],
        copyright:
          `Open Policy Agent is a <a href="https://www.cncf.io/">Cloud Native Computing Foundation</a> Graduated project.

<img src="${baseUrl}img/footer/cncf.svg" alt="CNCF Logo" style="max-width: 10rem; vertical-align: middle; margin: 0 10px;"><br />

© ${new Date().getFullYear()}
Open Policy Agent contributors.
<a href="https://github.com/open-policy-agent/opa/blob/main/LICENSE">Licensed under the Apache License, Version 2.0</a>.
See the <a href="${baseUrl}/docs/contributing">contributing documentation</a> for information about contributing.

The Linux Foundation has registered trademarks and uses trademarks. For a list of trademarks of The Linux Foundation, please see our Trademark Usage page.`,
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
        additionalLanguages: [
          "rego",
          "hcl",
          "json",
          "java",
          "scala",
          "gradle",
          "javadoc",
          "sql",
          "http",
          "diff",
          "typescript",
          "ini",
          "cypher",
          "csharp",
          "shell-session",
          "go-module",
          "docker",
          "javastacktrace",
          "properties",
          "log",
        ],
        magicComments: [
          {
            className: "code-block-terminal-command",
            line: "terminal-command",
          },
          {
            className: "code-block-terminal-command",
            line: "cmd",
          },
          {
            className: "code-block-diff-add-line",
            line: "diff-add",
            block: { start: "diff-add-start", end: "diff-add-end" },
          },
          {
            className: "code-block-diff-remove-line",
            line: "diff-remove",
            block: { start: "diff-remove-start", end: "diff-remove-end" },
          },
          {
            className: "theme-code-block-highlighted-line",
            line: "highlight-next-line",
            block: { start: "highlight-start", end: "highlight-end" },
          },
          {
            className: "code-block-error-line",
            line: "error-next-line",
            block: { start: "error-start", end: "error-end" },
          },
        ],
      },
      mermaid: {
        theme: { light: "base", dark: "dark" },
        options: {
          themeVariables: { // https://mermaid.js.org/config/theming.html#theme-variables
            fontFamily: "sans-serif",
            primaryColor: "#76d3ed",
            secondaryColor: "#fff",
            tertiaryColor: "#fff",
          },
        },
      },
    },

    plugins: [
      () => ({
        name: "raw-loader",
        configureWebpack() {
          return {
            module: {
              rules: [
                { test: /\.rego$/, use: "raw-loader" },
                { test: /\.txt$/, use: "raw-loader" },
              ],
            },
          };
        },
      }),
      async function ecosystemLanguagePageGen(context, options) {
        return {
          name: "ecosystem-language-gen",
          async loadContent() {
            const languages = await loadPages(path.join(context.siteDir, "src/data/ecosystem/languages/*.md"));
            return { languages };
          },

          async contentLoaded({ content, actions }) {
            const { pagesByLanguage, languages } = content;
            await Promise.all(
              Object.keys(languages).map(async (language) => {
                const routePath = path.join(baseUrl, `/ecosystem/by-language/${language}`);
                return actions.addRoute({
                  path: routePath,
                  component: require.resolve("./src/EcosystemLanguage.js"),
                  exact: true,
                  modules: {},
                  customData: { language },
                });
              }),
            );
          },
        };
      },

      async function ecosystemFeaturePageGen(context, options) {
        return {
          name: "ecosystem-feature-gen",
          async loadContent() {
            const features = await loadPages(path.join(context.siteDir, "src/data/ecosystem/features/*.md"));

            return { features };
          },

          async contentLoaded({ content, actions }) {
            const { features } = content;
            await Promise.all(
              Object.keys(features).map(async (feature) => {
                const routePath = path.join(baseUrl, `/ecosystem/by-feature/${feature}`);
                return actions.addRoute({
                  path: routePath,
                  component: require.resolve("./src/EcosystemFeature.js"),
                  exact: true,
                  modules: {},
                  customData: { feature },
                });
              }),
            );
          },
        };
      },

      async function ecosystemData(context, options) {
        return {
          name: "ecosystem-data",

          async loadContent() {
            const entries = await loadPages(path.join(context.siteDir, "src/data/ecosystem/entries/*.md"));
            const languages = await loadPages(path.join(context.siteDir, "src/data/ecosystem/languages/*.md"));
            const features = await loadPages(path.join(context.siteDir, "src/data/ecosystem/features/*.md"));
            const featureCategories = await loadPages(
              path.join(context.siteDir, "src/data/ecosystem/feature-categories/*.md"),
            );

            return {
              entries,
              languages,
              features,
              featureCategories,
            };
          },

          async contentLoaded({ content, actions }) {
            const { createData } = actions;
            const { entries, languages, features, featureCategories } = content;

            await createData("entries.json", JSON.stringify(entries, null, 2));
            await createData("languages.json", JSON.stringify(languages, null, 2));
            await createData("features.json", JSON.stringify(features, null, 2));
            await createData("feature-categories.json", JSON.stringify(featureCategories, null, 2));
          },
        };
      },

      async function builtinData(context, options) {
        return {
          name: "builtin-data",

          async loadContent() {
            const filePath = path.join(context.siteDir, "src", "data", "builtin_metadata.json");
            const fileContent = await fs.readFile(filePath, "utf-8");
            const builtins = JSON.parse(fileContent);
            return { builtins };
          },

          async contentLoaded({ content, actions }) {
            const { createData } = actions;
            const { builtins } = content;

            await createData("builtins.json", JSON.stringify(builtins, null, 2));
          },
        };
      },

      async function ecosystemPagesGen(context, options) {
        return {
          name: "ecosystem-entries-pages-gen",
          async loadContent() {
            const entries = await loadPages(path.join(context.siteDir, "src/data/ecosystem/entries/*.md"));
            return { entries };
          },

          async contentLoaded({ content, actions }) {
            const { entries } = content;

            await Promise.all(
              Object.values(entries).map(async (entry) => {
                const routePath = path.join(baseUrl, `/ecosystem/entry/${entry.id}`);
                return actions.addRoute({
                  path: routePath,
                  component: require.resolve("./src/EcosystemEntry.js"),
                  exact: true,
                  modules: {},
                  customData: { id: entry.id },
                });
              }),
            );
          },
        };
      },
    ],
    clientModules: [
      require.resolve("./src/lib/playground.js"),
    ],
    stylesheets: [
      {
        href: "https://unpkg.com/@antonz/codapi@0.19.8/dist/snippet.css",
      },
    ],
    scripts: [
      {
        src: "https://unpkg.com/@antonz/codapi@0.19.8/dist/snippet.js",
        defer: true,
      },
    ],
  }
);
