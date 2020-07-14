package opa.example

import data.servers
import data.networks
import data.ports

public_servers[server] { server = servers[_]; server.ports[_] = ports[i].id; ports[i].networks[_] = networks[j].id; networks[j].public = true }
violations[server] { server = servers[_]; server.protocols[_] = "http"; public_servers[server] }