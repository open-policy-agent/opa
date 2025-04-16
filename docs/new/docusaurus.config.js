const { themes } = require("prism-react-renderer");
const lightCodeTheme = themes.github;
const darkCodeTheme = themes.dracula;
const semver = require("semver");

import fs from "fs/promises";
import glob from "glob";
import { matter } from "md-front-matter";
import path from "path";

// With JSDoc @type annotations, IDEs can provide config autocompletion
/** @type {import("@docusaurus/types").DocusaurusConfig} */
(
  module.exports = {
    title: "Open Policy Agent",
    tagline: "Policy-based control for cloud native environments",
    url: "https://openpolicyagent.org",
    baseUrl: "/",
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
            routeBasePath: "/",
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
          src: "img/logo.png",
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
        <a href="https://github.com/open-policy-agent/gatekeeper"
           target="_blank"
           rel="noopener noreferrer"
           aria-label="GitHub repository">
          <img src="/img/community-logos/github.png" alt="GitHub" style="width: 24px; height: auto; margin-left: 8px;" />
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
          <img src="/img/community-logos/slack.png" alt="Slack" style="width: 24px; height: auto; margin-left: 8px;" />
        </a>
      `,
          },
        ],
      },
      footer: {
        style: "dark",
        links: [],
        copyright: `© ${
          new Date().getFullYear()
        } Open Policy Agent contributors. Licensed under the Apache License, Version 2.0. See the contributing documentation for information about contributing.

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
      async function ecosystemIndexPageGen(context, options) {
        return {
          name: "ecosystem-index-gen",
          async loadContent() {
            const entryGlob = path.resolve(__dirname, "src/data/ecosystem/entries/*.md");
            const logoGlobRoot = path.resolve(__dirname, "static/img/ecosystem/logos");

            const files = await new Promise((resolve, reject) => {
              glob(entryGlob, (err, matches) => {
                if (err) reject(err);
                else resolve(matches);
              });
            });

            const pages = await files.reduce(async (accPromise, filePath) => {
              const acc = await accPromise;
              const content = await fs.readFile(filePath, "utf-8");
              const parsed = matter(content);

              const id = path.parse(filePath).name;

              // Check if a matching logo file exists
              const logoFiles = await new Promise((resolve, reject) => {
                glob(`${logoGlobRoot}/${id}*`, (err, matches) => {
                  if (err) reject(err);
                  else resolve(matches);
                });
              });

              const logoPath = logoFiles.length > 0
                ? `/img/ecosystem/logos/${path.basename(logoFiles[0])}`
                : "/img/logo.png";

              acc[id] = {
                ...parsed.data,
                content: parsed.content,
                filePath,
                id,
                logo: logoPath,
              };

              return acc;
            }, Promise.resolve({}));

            return { pages };
          },

          async contentLoaded({ content, actions }) {
            actions.addRoute({
              path: "/ecosystem",
              component: require.resolve("./src/EcosystemIndex.js"),
              exact: true,
              modules: {},
              customData: { content },
            });
          },
        };
      },
      async function ecosystemPagesGen(context, options) {
        return {
          name: "ecosystem-entries-pages-gen",
          async loadContent() {
            const entryGlob = path.resolve(__dirname, "src/data/ecosystem/entries/*.md");
            const logoGlobRoot = path.resolve(__dirname, "static/img/ecosystem/logos");

            const files = await new Promise((resolve, reject) => {
              glob(entryGlob, (err, matches) => {
                if (err) reject(err);
                else resolve(matches);
              });
            });

            const pages = await files.reduce(async (accPromise, filePath) => {
              const acc = await accPromise;
              const content = await fs.readFile(filePath, "utf-8");
              const parsed = matter(content);

              const id = path.parse(filePath).name;

              // Check if a matching logo file exists
              const logoFiles = await new Promise((resolve, reject) => {
                glob(`${logoGlobRoot}/${id}*`, (err, matches) => {
                  if (err) reject(err);
                  else resolve(matches);
                });
              });

              const logoPath = logoFiles.length > 0
                ? `/img/ecosystem/logos/${path.basename(logoFiles[0])}`
                : "/img/logo.png";

              acc[id] = {
                ...parsed.data,
                content: parsed.content,
                filePath,
                id,
                logo: logoPath,
              };

              return acc;
            }, Promise.resolve({}));

            return { pages };
          },

          async contentLoaded({ content, actions }) {
            const { pages } = content;
            await Promise.all(
              Object.values(pages).map(async (page) => {
                const routePath = `/ecosystem/${page.id}`;
                return actions.addRoute({
                  path: routePath,
                  component: require.resolve("./src/EcosystemEntry.js"),
                  exact: true,
                  modules: {},
                  customData: { ...page },
                });
              }),
            );
          },
        };
      },
      require.resolve("docusaurus-lunr-search"),
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
