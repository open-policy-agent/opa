package envoy.http.public

headers := input.attributes.request.http.headers

default allow := false

allow if {
	input.attributes.request.http.method == "GET"
	input.attributes.request.http.path == "/"
}

allow if headers.authorization == "Basic charlie"
