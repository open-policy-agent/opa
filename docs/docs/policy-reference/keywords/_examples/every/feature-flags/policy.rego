package play

default allow := false

allow if {
	input.path == "/new/checkout"

	every feature in new_checkout_features {
		input.features[feature] == true
	}
}

new_checkout_features := {
	"new_ui",
	"test_speedy_checkout",
}
