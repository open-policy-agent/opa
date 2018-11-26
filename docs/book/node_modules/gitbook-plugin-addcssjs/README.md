# gitbook-plugin-addcssjs
Plugin for gitbook for adding external css and js files to the git book.

## How to use
- Add addcssjs plugin to your book.json:


    "plugins": [
      "addcssjs"
    ]
- Add plugin configuration specifying css and js files to include:


    "pluginsConfig": {
      "addcssjs": {
        "js": ["js/custom.js"],
        "css": ["css/custom.css"]
      }
    }