package test

import future.keywords.not

paren_set {
	not ({x})
}

paren_object {
	not ({"a": 1})
}

paren_ref {
	not (input.x)
}

explicit_body {
	not { x }
}
