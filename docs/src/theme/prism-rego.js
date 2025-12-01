// Custom Rego language definition for Prism.js
// Based on rego.tmLanguage from the OPA repository
//
// Note: As of December 2025, https://github.com/PrismJS/prism is not accepting
// language updates until they cut v2, so we maintain our own custom definition here.

export default function(Prism) {
  Prism.languages.rego = {
    "comment": {
      pattern: /#.*/,
      greedy: true,
    },

    // Keywords from tmLanguage: default, not, package, import, as, with, else, some, in, every, if, contains
    "keyword": {
      pattern: /\b(?:default|not|package|import|as|with|else|some|in|every|if|contains)\b/,
      greedy: true,
    },

    // Boolean and null constants
    "boolean": /\b(?:true|false)\b/,
    "null": /\bnull\b/,

    // Template/interpolated strings
    "template-string": {
      pattern: /\$`(?:[^`\\]|\\[\s\S])*`/,
      greedy: true,
      inside: {
        "interpolation": {
          pattern: /\{[^}]+\}/,
          inside: {
            "interpolation-punctuation": {
              pattern: /^\{|\}$/,
              alias: "punctuation",
            },
            rest: null, // Will be populated later
          },
        },
        "string": /[\s\S]/,
      },
    },

    // Interpolated double quoted strings
    "interpolated-string": {
      pattern: /\$"(?:[^"\\]|\\[\s\S])*"/,
      greedy: true,
      inside: {
        "interpolation": {
          pattern: /\{[^}]+\}/,
          inside: {
            "interpolation-punctuation": {
              pattern: /^\{|\}$/,
              alias: "punctuation",
            },
            rest: null, // Will be populated later
          },
        },
        "escape": /\\(?:["\\\/bfnrt]|u[0-9a-fA-F]{4})/,
        "string": /[\s\S]/,
      },
    },

    // Regular strings
    "string": [
      {
        pattern: /"(?:[^"\\]|\\[\s\S])*"/,
        greedy: true,
        inside: {
          "escape": /\\(?:["\\\/bfnrt]|u[0-9a-fA-F]{4})/,
        },
      },
      {
        pattern: /`[^`]*`/,
        greedy: true,
      },
    ],

    // Function calls
    "function": {
      pattern: /\b[a-zA-Z_][a-zA-Z0-9_]*(?:\s*\.\s*[a-zA-Z_][a-zA-Z0-9_]*)*(?=\s*\()/,
      inside: {
        "namespace": {
          pattern: /\b\w+\b(?=\s*\.)/,
          alias: "punctuation",
        },
        "punctuation": /\./,
      },
    },

    // Numbers (including scientific notation)
    "number": {
      pattern: /-?\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/,
      greedy: true,
    },

    // Variables and identifiers
    "variable": {
      pattern: /\b[a-zA-Z_][a-zA-Z0-9_]*\b/,
      greedy: true,
    },

    // Comparison and arithmetic operators
    "operator": /==|!=|<=|>=|[<>+\-*/%|&]|:=|=/,

    // Assignment operators
    "assignment": /:=|=/,

    // Punctuation
    "punctuation": /[;,.\[\]{}()]/,
  };

  // Copy the main language definition to interpolation rest for recursive parsing
  Prism.languages.rego["template-string"].inside["interpolation"].inside.rest = Prism.languages.rego;
  Prism.languages.rego["interpolated-string"].inside["interpolation"].inside.rest = Prism.languages.rego;
}
