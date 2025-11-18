const { themes } = require("prism-react-renderer");
const lightCodeTheme = themes.github;
const darkCodeTheme = themes.dracula;
const semver = require("semver");
import fs from "fs/promises";
const path = require("path");

import { loadPages } from "./src/lib/ecosystem/loadPages.js";
import { loadRules } from "./src/lib/projects/regal/loadRules.js";

const baseUrl = "/";

// With JSDoc @type annotations, IDEs can provide config autocompletion
/** @type {import("@docusaurus/types").DocusaurusConfig} */
(
  module.exports = {
    title: "Open Policy Agent",
    tagline: "Policy-based control for cloud native environments",
    url: "https://openpolicyagent.org",
    baseUrl: baseUrl,
    trailingSlash: false,
    // when BUILD_VERSION is set (release builds), warn on broken links/anchors so we don't break main
    // when not set (PR checks), throw to flag issues for developers
    onBrokenLinks: process.env.BUILD_VERSION ? 'warn' : 'throw',
    onBrokenAnchors: process.env.BUILD_VERSION ? 'warn' : 'throw',
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

    customFields: {
      buildVersion: process.env.BUILD_VERSION,
    },

    markdown: {
      hooks: { onBrokenMarkdownLinks: "throw" },
      mermaid: true,
    },
    themes: ["@docusaurus/theme-mermaid"],

    themeConfig: {
      announcementBar: {
        id: 'opa_2025_survey',
        content:
          'Help shape OPA\'s future! Take the <a target="_blank" rel="noopener noreferrer" href="https://www.surveymonkey.com/r/SCBSDZN">2025 OPA Community Survey</a> 🚀',
        backgroundColor: '#ff8c42',
        textColor: '#ffffff',
        isCloseable: false,
      },
      colorMode: {
        defaultMode: "light",
        disableSwitch: false,
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
            type: "image/png",
            href: "/favicon-96x96.png",
            sizes: "96x96",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "icon",
            type: "image/svg+xml",
            href: "/favicon.svg",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "shortcut icon",
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
          tagName: "meta",
          attributes: {
            name: "apple-mobile-web-app-title",
            content: "OPA",
          },
        },
        {
          tagName: "link",
          attributes: {
            rel: "manifest",
            href: "/site.webmanifest",
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
          {
            // Based on implementation outlined here:
            // https://github.com/facebook/docusaurus/issues/7227#issue-1212117180
            type: "custom-currentVersionNavbarItem",
            position: "left",
          },
          { to: "/docs/", label: "Docs", position: "right" },
          {
            type: "dropdown",
            label: "Resources",
            position: "right",
            items: [
              { to: "/security", label: "Security" },
              { to: "/support", label: "Support" },
              { to: "/community", label: "Community" },
              { href: "https://blog.openpolicyagent.org/", label: "Blog" },
            ],
          },
          {
            type: "dropdown",
            label: "Projects",
            position: "right",
            items: [
              { to: "/docs", label: "OPA" },
              { to: "/projects/regal", label: "Regal" },
              {
                type: "html",
                value: "<hr style=\"margin: 0.3rem 1rem\">",
              },
              { href: "https://open-policy-agent.github.io/gatekeeper/website/", label: "OPA Gatekeeper" },
              { href: "https://www.conftest.dev", label: "Conftest" },
            ],
          },
          { to: "/ecosystem/", label: "Ecosystem", position: "right" },
          { href: "https://play.openpolicyagent.org/", label: "Play", position: "right" },
          {
            type: "html",
            position: "right",
            value: `
        <a href="https://github.com/open-policy-agent"
           target="_blank"
           rel="noopener noreferrer"
           aria-label="GitHub repository">
          <img src="${
              path.join(baseUrl, "img/nav/github-light.svg")
            }" class="light-only" alt="GitHub" style="width: 24px; height: auto; margin-left: 8px;" />
          <img src="${
              path.join(baseUrl, "img/nav/github-dark.svg")
            }" class="dark-only" alt="GitHub" style="width: 24px; height: auto; margin-left: 8px;" />
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
          <img src="${
              path.join(baseUrl, "img/nav/slack-light.svg")
            }" class="light-only" alt="Slack" style="width: 24px; height: auto; margin-left: 8px;" />
          <img src="${
              path.join(baseUrl, "img/nav/slack-dark.svg")
            }" class="dark-only" alt="Slack" style="width: 24px; height: auto; margin-left: 8px;" />
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

<img src="${
            path.join(baseUrl, "img/footer/cncf-light.svg")
          }" alt="CNCF Logo" class="light-only" style="max-width: 10rem; vertical-align: middle; margin: 0 10px;">
<img src="${
            path.join(baseUrl, "img/footer/cncf-dark.svg")
          }" alt="CNCF Logo" class="dark-only" style="max-width: 10rem; vertical-align: middle; margin: 0 10px;">
<br />

© ${new Date().getFullYear()}
Open Policy Agent contributors.
<a href="https://github.com/open-policy-agent/opa/blob/main/LICENSE">Licensed under the Apache License, Version 2.0</a>.
See the <a href="${
            path.join(baseUrl, "/docs/contributing")
          }">contributing documentation</a> for information about contributing.

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
      [
        "@docusaurus/plugin-google-gtag",
        {
          trackingID: "G-JNBNV64PDX",
          anonymizeIP: true,
        },
      ],
      [
        "@docusaurus/plugin-content-docs",
        {
          id: "regal",
          path: "projects/regal",
          routeBasePath: "projects/regal",
          sidebarPath: require.resolve("./src/lib/sidebar-auto.js"),
        },
      ],
      [
        require.resolve("@easyops-cn/docusaurus-search-local"),
        {
          indexPages: true,
        },
      ],
      () => ({
        name: "raw-loader",
        configureWebpack() {
          return {
            module: {
              rules: [
                { test: /\.rego$/, use: "raw-loader" },
                { test: /\.mermaid$/, use: "raw-loader" },
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
            const filePath = "../builtin_metadata.json";
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

      async function versionsData(context, options) {
        return {
          name: "versions-data",

          async loadContent() {
            const capabilitiesDir = path.resolve(__dirname, "../capabilities");
            let sortedVersions = [];

            const dirents = await fs.readdir(capabilitiesDir, { withFileTypes: true });

            const versionStrings = dirents
              .filter(dirent => dirent.isFile() && dirent.name.endsWith(".json"))
              .map(dirent => dirent.name.replace(".json", ""));

            const validVersions = versionStrings.filter(v => semver.valid(v));

            sortedVersions = semver.sort(validVersions);

            return { versions: sortedVersions };
          },

          async contentLoaded({ content, actions }) {
            const { createData } = actions;
            const { versions } = content;

            await createData("versions.json", JSON.stringify(versions, null, 2));

            const staticDir = path.join(context.siteDir, "static", "data");
            await fs.mkdir(staticDir, { recursive: true });
            await fs.writeFile(
              path.join(staticDir, "versions.json"),
              JSON.stringify(versions, null, 2),
            );
          },
        };
      },

      async function cliData(context, options) {
        return {
          name: "cli-data",

          async loadContent() {
            const filePath = path.join(context.siteDir, "src/data/cli.json");
            const cliJson = await fs.readFile(filePath, "utf-8");
            const parsedData = JSON.parse(cliJson);

            return parsedData;
          },

          async contentLoaded({ content, actions }) {
            const { createData } = actions;

            await createData("cli.json", JSON.stringify(content, null, 2));
          },
        };
      },

      async function versionsPageGen(context, options) {
        return {
          name: "version-page-gen",
          async contentLoaded({ content, actions }) {
            return actions.addRoute({
              path: path.join(baseUrl, `/docs/archive`),
              component: require.resolve("./src/Archive.js"),
              exact: true,
              modules: {},
            });
          },
        };
      },

      async function ecosystemData(context, options) {
        return {
          name: "regal",

          async loadContent() {
            const rules = await loadRules();

            return { rules };
          },

          async contentLoaded({ content, actions }) {
            await actions.createData("rules.json", JSON.stringify(content.rules, null, 2));
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
