var test = require("tape");
var escape = require("./");

test("escaping", function(t) {
	t.equal(escape("no escape"), "no escape");
	t.equal(escape("foo&bar"), "foo&amp;bar");
	t.equal(escape("<tag>"), "&lt;tag&gt;");
	t.equal(escape("test=\'foo\'"), "test=&#39;foo&#39;");
	t.equal(escape("test=\"foo\""), "test=&quot;foo&quot;");
	t.equal(escape("<ta'&g\">"), "&lt;ta&#39;&amp;g&quot;&gt;");
	t.end();
});
