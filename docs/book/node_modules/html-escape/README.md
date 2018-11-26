# html-escape

Escape a string to be safe for use in HTML by escaping [reserved
characters](http://www.w3.org/International/questions/qa-escapes#use)
(`&<>'"`).

# Example

```js
> var escape = require("html-escape");
> var xssAttempt = "Hello <script>while(1);</script> world!";
> // Output safe html
> console.log("<p>" + escape(xssAttempt) + "</p>");
"<p>Hello &lt;script&gt;while(1);&lt;/script&gt; world!</p>"
```

# Installation

```
npm install html-escape
```
