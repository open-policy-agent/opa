// Codapi's basic editor makes the code block contenteditable and, the first
// time a snippet is focused, replaces the highlighted markup with plain text so
// that editing doesn't mangle the Prism token elements (see initEditor in
// https://github.com/nalgeon/codapi-js/blob/main/src/editor.js). Re-apply the
// highlighting after every edit instead, using the same Prism instance and theme
// Docusaurus uses for static code blocks.

import { useEffect } from "react";

import { usePrismTheme } from "@docusaurus/theme-common";
import { normalizeTokens, Prism } from "prism-react-renderer";

const ELEMENT_NODE = 1;
const TEXT_NODE = 3;

// Elements a browser may insert into a contenteditable to start a new line.
const BLOCK_ELEMENTS = new Set(["DIV", "P"]);

// containerRef must point at the element wrapping the codapi-snippet, since that
// is how codapi itself locates the code.
export default function useSnippetHighlighting(containerRef) {
  const theme = usePrismTheme();

  useEffect(() => {
    const container = containerRef.current;
    // Docusaurus remounts a code block once hydration is done (CodeBlock renders
    // key={String(isBrowser)}), which replaces the element resolved here, while
    // codapi binds to the replacement. So listen on an ancestor and resolve the
    // code element per event - focusin rather than focus, as focus doesn't
    // bubble.
    const root = container?.parentElement;
    if (!root) {
      return;
    }

    const stylesByLanguage = new Map();

    // target returns what to highlight for an event, or null when the event
    // belongs to another snippet under the same ancestor.
    const target = (event) => {
      const code = findCodeElement(container);
      if (!code || !(code === event.target || code.contains(event.target))) {
        return null;
      }
      const language = findLanguage(code);
      if (!language || !Prism.languages[language]) {
        return null;
      }
      if (!stylesByLanguage.has(language)) {
        stylesByLanguage.set(language, tokenStyles(theme, language));
      }
      return { code, language, styles: stylesByLanguage.get(language) };
    };

    // fallbackCaret is used when the caret is no longer readable from the
    // document selection, which is the case once codapi has replaced the
    // contents with plain text.
    const rehighlight = ({ code, language, styles }, fallbackCaret = null) => {
      const position = caretPosition(code);
      const { text, caret } = readCode(code, position?.node, position?.offset);
      const lines = highlight(text, language);
      if (!lines) {
        return;
      }
      render(code, lines, styles);
      code.dataset.snippetHighlighted = "true";

      const offset = caret ?? fallbackCaret;
      if (offset !== null && document.activeElement === code) {
        placeCaret(code, offset);
      }
    };

    let composing = false;

    const onFocusIn = (event) => {
      const found = target(event);
      // Codapi only drops the highlighting on the first focus.
      if (!found || found.code.dataset.snippetHighlighted) {
        return;
      }
      // Read the caret before codapi replaces the contents, which discards the
      // selection - this is our only chance to keep the cursor where the user
      // clicked. Codapi's own handler runs synchronously, so wait for it.
      const position = caretPosition(found.code);
      const caret = position ? readCode(found.code, position.node, position.offset).caret : null;
      window.setTimeout(() => rehighlight(found, caret), 0);
    };

    const onInput = (event) => {
      const found = target(event);
      // Re-rendering mid-composition would cancel the input method.
      if (found && !composing) {
        rehighlight(found);
      }
    };

    const onCompositionStart = (event) => {
      if (target(event)) {
        composing = true;
      }
    };

    const onCompositionEnd = (event) => {
      const found = target(event);
      if (found) {
        composing = false;
        rehighlight(found);
      }
    };

    root.addEventListener("focusin", onFocusIn);
    root.addEventListener("input", onInput);
    root.addEventListener("compositionstart", onCompositionStart);
    root.addEventListener("compositionend", onCompositionEnd);

    // Docusaurus recolors static code blocks when the color mode changes, but it
    // no longer owns the contents of a snippet that has been edited.
    const edited = findCodeElement(container);
    if (edited?.dataset.snippetHighlighted) {
      const language = findLanguage(edited);
      if (language && Prism.languages[language]) {
        rehighlight({ code: edited, language, styles: tokenStyles(theme, language) });
      }
    }

    return () => {
      root.removeEventListener("focusin", onFocusIn);
      root.removeEventListener("input", onInput);
      root.removeEventListener("compositionstart", onCompositionStart);
      root.removeEventListener("compositionend", onCompositionEnd);
    };
  }, [containerRef, theme]);
}

// Snippet output doesn't go through a Docusaurus code block - RunSnippet renders
// it in a plain pre, and so does codapi - so highlight it here to match the
// snippet it belongs to. useJsonHighlighting covers the output rendered before a
// run, useResultHighlighting the output of each run.
export function useJsonHighlighting(elementRef, text) {
  const theme = usePrismTheme();

  useEffect(() => {
    if (elementRef.current && text) {
      highlightJson(elementRef.current, text, theme);
    }
  }, [elementRef, text, theme]);
}

export function useResultHighlighting(snippet) {
  const theme = usePrismTheme();

  useEffect(() => {
    if (!snippet) {
      return;
    }
    // Codapi dispatches 'result' once it has rendered the output.
    const onResult = () => {
      const output = snippet.querySelector("codapi-output code") ?? snippet.querySelector("codapi-output");
      if (output) {
        highlightJson(output, output.textContent, theme);
      }
    };
    snippet.addEventListener("result", onResult);
    return () => snippet.removeEventListener("result", onResult);
  }, [snippet, theme]);
}

// highlightJson leaves the element alone when its content isn't JSON, which is
// the case for an evaluation error.
function highlightJson(element, text, theme) {
  try {
    JSON.parse(text);
  } catch {
    return;
  }
  const lines = highlight(text, "json");
  if (lines) {
    render(element, lines, tokenStyles(theme, "json"));
  }
}

// Mirrors codapi's own lookup.
function findCodeElement(container) {
  const previous = container?.previousElementSibling;
  if (!previous) {
    return null;
  }
  return previous.querySelector("code") ?? previous;
}

// Docusaurus records the language as a language-* class on the pre element.
function findLanguage(code) {
  const pre = code.closest("pre") ?? code.parentElement;
  const match = /(?:^|\s)language-([\w-]+)/.exec(pre?.className ?? "");
  return match?.[1];
}

// Mirrors prism-react-renderer's themeToDict.
function tokenStyles(theme, language) {
  const styles = {};
  for (const { types, style, languages } of theme.styles) {
    if (languages && !languages.includes(language)) {
      continue;
    }
    for (const type of types) {
      styles[type] = { ...styles[type], ...style };
    }
  }
  return styles;
}

// Mirrors prism-react-renderer's useTokenize.
function highlight(code, language) {
  const env = { code, grammar: Prism.languages[language], language, tokens: [] };
  if (!env.grammar) {
    return null;
  }
  Prism.hooks.run("before-tokenize", env);
  env.tokens = Prism.tokenize(env.code, env.grammar);
  Prism.hooks.run("after-tokenize", env);
  return normalizeTokens(env.tokens);
}

// Unlike Docusaurus, we don't wrap each line in an element: keeping the line
// breaks as plain text means the snippet still reads back correctly through
// textContent, which is how codapi collects the code to evaluate.
function render(code, lines, styles) {
  const fragment = document.createDocumentFragment();

  lines.forEach((line, index) => {
    if (index > 0) {
      fragment.append(document.createTextNode("\n"));
    }
    for (const token of line) {
      // Empty lines are a single newline token, and the newlines between lines
      // are added above.
      if (token.empty || token.content === "") {
        continue;
      }
      fragment.append(renderToken(token, styles));
    }
  });

  code.replaceChildren(fragment);
}

function renderToken(token, styles) {
  const style = token.types.length === 1 && token.types[0] === "plain"
    ? null
    : Object.assign({}, ...token.types.map((type) => styles[type]));

  if (!style) {
    return document.createTextNode(token.content);
  }

  const span = document.createElement("span");
  span.className = ["token", ...token.types].join(" ");
  for (const [property, value] of Object.entries(style)) {
    if (value !== undefined) {
      span.style[property] = value;
    }
  }
  span.textContent = token.content;
  return span;
}

function caretPosition(code) {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) {
    return null;
  }
  const range = selection.getRangeAt(0);
  if (!code.contains(range.endContainer)) {
    return null;
  }
  return { node: range.endContainer, offset: range.endOffset };
}

// The line breaks can't be taken from textContent: both Docusaurus and the
// browser (when the user presses Enter) express them as elements.
function readCode(root, caretNode, caretOffset) {
  let text = "";
  let caret = null;

  const visit = (node) => {
    const children = node.childNodes;

    for (let i = 0; i < children.length; i++) {
      if (node === caretNode && i === caretOffset) {
        caret = text.length;
      }

      const child = children[i];
      if (child.nodeType === TEXT_NODE) {
        if (child === caretNode) {
          caret = text.length + Math.min(caretOffset, child.data.length);
        }
        text += child.data;
      } else if (child.nodeName === "BR") {
        text += "\n";
      } else if (child.nodeType === ELEMENT_NODE) {
        if (BLOCK_ELEMENTS.has(child.nodeName) && text !== "" && !text.endsWith("\n")) {
          text += "\n";
        }
        visit(child);
      }
    }

    if (node === caretNode && caretOffset >= children.length) {
      caret = text.length;
    }
  };

  visit(root);

  return { text, caret };
}

function placeCaret(code, offset) {
  const selection = window.getSelection();
  if (!selection) {
    return;
  }

  let remaining = offset;
  let target = null;

  const visit = (node) => {
    for (const child of node.childNodes) {
      if (target) {
        return;
      }
      if (child.nodeType === TEXT_NODE) {
        if (remaining <= child.data.length) {
          target = { node: child, offset: remaining };
          return;
        }
        remaining -= child.data.length;
      } else if (child.nodeType === ELEMENT_NODE) {
        visit(child);
      }
    }
  };

  visit(code);

  const range = document.createRange();
  if (target) {
    range.setStart(target.node, target.offset);
    range.collapse(true);
  } else {
    // The offset is past the end of the content.
    range.selectNodeContents(code);
    range.collapse(false);
  }

  selection.removeAllRanges();
  selection.addRange(range);
}
