package opa.examples

import data.servers

violations[server] {
	server = servers[_]
	server.protocols[_] = "http"
	public_servers[server]
}
