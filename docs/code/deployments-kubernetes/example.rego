package example

greeting = msg {
    info := opa.runtime()
    hostname := info.env["HOSTNAME"] # Kubernetes sets the HOSTNAME environment variable.
    msg := sprintf("hello from pod %q!", [hostname])
}