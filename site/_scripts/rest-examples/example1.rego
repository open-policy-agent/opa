package opa.examples

import data.servers
import data.networks
import data.ports

public_servers[server] {
	server = servers[_]
	server.ports[_] = ports[k].id
	ports[k].networks[_] = networks[m].id
	networks[m].public = true
}
