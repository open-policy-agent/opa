var escape = require("./index");
var xssAttempt = "Hello <script>while(1);</script> world!";
// Output safe html
console.log("<p>" + escape(xssAttempt) + "</p>");
