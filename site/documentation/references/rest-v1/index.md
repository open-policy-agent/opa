---
nav_id: MAIN_DOCUMENTATION
doc_id: REST_API_VERSION_1
layout: documentation

title: REST API (V1)
---

{% contentfor header %}

# REST API (V1)

This document is the authoritative specification of the OPA REST API (v1).

{% endcontentfor %}

{% contentfor body %}

## <a name="policy-api"></a> Policy API

The Policy API exposes CRUD endpoints for managing policy modules. Policy modules can be added, removed, and modified at any time.

The identifiers given to policy modules are only used for management purposes. They are not used outside of the Policy API.

### List Policies

```
GET /v1/policies
```

List policy modules.

#### Example Request

```http
GET /v1/policies HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "id": "example2",
      "raw": "package opa.examples\n\nimport data.servers\n\nviolations[server] {\n\tserver = servers[_]\n\tserver.protocols[_] = \"http\"\n\tpublic_servers[server]\n}\n",
      "ast": {
        "package": {
          "path": [
            {
              "type": "var",
              "value": "data"
            },
            {
              "type": "string",
              "value": "opa"
            },
            {
              "type": "string",
              "value": "examples"
            }
          ]
        },
        "rules": [
          {
            "head": {
              "name": "violations",
              "key": {
                "type": "var",
                "value": "server"
              }
            },
            "body": [
              {
                "index": 0,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "var",
                    "value": "server"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "servers"
                      },
                      {
                        "type": "var",
                        "value": "$0"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 1,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "server"
                      },
                      {
                        "type": "string",
                        "value": "protocols"
                      },
                      {
                        "type": "var",
                        "value": "$1"
                      }
                    ]
                  },
                  {
                    "type": "string",
                    "value": "http"
                  }
                ]
              },
              {
                "index": 2,
                "terms": {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "opa"
                    },
                    {
                      "type": "string",
                      "value": "examples"
                    },
                    {
                      "type": "string",
                      "value": "public_servers"
                    },
                    {
                      "type": "var",
                      "value": "server"
                    }
                  ]
                }
              }
            ]
          }
        ]
      }
    },
    {
      "id": "example1",
      "raw": "package opa.examples\n\nimport data.servers\nimport data.networks\nimport data.ports\n\npublic_servers[server] {\n\tserver = servers[_]\n\tserver.ports[_] = ports[k].id\n\tports[k].networks[_] = networks[m].id\n\tnetworks[m].public = true\n}\n",
      "ast": {
        "package": {
          "path": [
            {
              "type": "var",
              "value": "data"
            },
            {
              "type": "string",
              "value": "opa"
            },
            {
              "type": "string",
              "value": "examples"
            }
          ]
        },
        "rules": [
          {
            "head": {
              "name": "public_servers",
              "key": {
                "type": "var",
                "value": "server"
              }
            },
            "body": [
              {
                "index": 0,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "var",
                    "value": "server"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "servers"
                      },
                      {
                        "type": "var",
                        "value": "$0"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 1,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "server"
                      },
                      {
                        "type": "string",
                        "value": "ports"
                      },
                      {
                        "type": "var",
                        "value": "$1"
                      }
                    ]
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "ports"
                      },
                      {
                        "type": "var",
                        "value": "k"
                      },
                      {
                        "type": "string",
                        "value": "id"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 2,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "ports"
                      },
                      {
                        "type": "var",
                        "value": "k"
                      },
                      {
                        "type": "string",
                        "value": "networks"
                      },
                      {
                        "type": "var",
                        "value": "$2"
                      }
                    ]
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "networks"
                      },
                      {
                        "type": "var",
                        "value": "m"
                      },
                      {
                        "type": "string",
                        "value": "id"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 3,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "networks"
                      },
                      {
                        "type": "var",
                        "value": "m"
                      },
                      {
                        "type": "string",
                        "value": "public"
                      }
                    ]
                  },
                  {
                    "type": "boolean",
                    "value": true
                  }
                ]
              }
            ]
          }
        ]
      }
    }
  ]
}

```

#### Status Codes

- **200** - no error
- **500** - server error

### Get a Policy

```
GET /v1/policies/<id>
```

Get a policy module.

#### Example Request

```http
GET /v1/policies/example1 HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": {
    "id": "example1",
    "raw": "package opa.examples\n\nimport data.servers\nimport data.networks\nimport data.ports\n\npublic_servers[server] {\n\tserver = servers[_]\n\tserver.ports[_] = ports[k].id\n\tports[k].networks[_] = networks[m].id\n\tnetworks[m].public = true\n}\n",
    "ast": {
      "package": {
        "path": [
          {
            "type": "var",
            "value": "data"
          },
          {
            "type": "string",
            "value": "opa"
          },
          {
            "type": "string",
            "value": "examples"
          }
        ]
      },
      "rules": [
        {
          "head": {
            "name": "public_servers",
            "key": {
              "type": "var",
              "value": "server"
            }
          },
          "body": [
            {
              "index": 0,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "var",
                  "value": "server"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "servers"
                    },
                    {
                      "type": "var",
                      "value": "$0"
                    }
                  ]
                }
              ]
            },
            {
              "index": 1,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "server"
                    },
                    {
                      "type": "string",
                      "value": "ports"
                    },
                    {
                      "type": "var",
                      "value": "$1"
                    }
                  ]
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "ports"
                    },
                    {
                      "type": "var",
                      "value": "k"
                    },
                    {
                      "type": "string",
                      "value": "id"
                    }
                  ]
                }
              ]
            },
            {
              "index": 2,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "ports"
                    },
                    {
                      "type": "var",
                      "value": "k"
                    },
                    {
                      "type": "string",
                      "value": "networks"
                    },
                    {
                      "type": "var",
                      "value": "$2"
                    }
                  ]
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "networks"
                    },
                    {
                      "type": "var",
                      "value": "m"
                    },
                    {
                      "type": "string",
                      "value": "id"
                    }
                  ]
                }
              ]
            },
            {
              "index": 3,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "networks"
                    },
                    {
                      "type": "var",
                      "value": "m"
                    },
                    {
                      "type": "string",
                      "value": "public"
                    }
                  ]
                },
                {
                  "type": "boolean",
                  "value": true
                }
              ]
            }
          ]
        }
      ]
    }
  }
}
```

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **404** - not found
- **500** - server error

### Create or Update a Policy

```
PUT /v1/policies/<id>
Content-Type: text/plain
```

Create or update a policy module.

If the policy module does not exist, it is created. If the policy module already exists, it is replaced.

#### Example Request

```http
PUT /v1/policies/example1 HTTP/1.1
Content-Type: text/plain
```

```ruby
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
```

> cURL's `-d/--data` flag removes newline characters from input files. Use the `--data-binary` flag instead.
{: .opa-tip}

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{}
```

### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **metrics** - Return compiler performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

Before accepting the request, the server will parse, compile, and install the policy module. If the policy module is invalid, one of these steps will fail and the server will respond with 400. The error message in the response will be set to indicate the source of the error.

### Delete a Policy

```
DELETE /v1/policies/<id>
```

Delete a policy module.

#### Example Request

```http
DELETE /v1/policies/example2 HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{}
```

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **metrics** - Return compiler performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **204** - no content
- **400** - bad request
- **404** - not found
- **500** - server error

If other policy modules in the same package depend on rules in the policy module to be deleted, the server will return 400.

## <a name="data-api"></a> Data API

The Data API exposes endpoints for reading and writing documents in OPA. For an introduction to the different types of documents in OPA see [How Does OPA Work?](../../how-does-opa-work/).

### Get a Document

```
GET /v1/data/{path:.+}
```

Get a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

#### Example Request

```http
GET /v1/data/opa/examples/public_servers HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "id": "s1",
      "name": "app",
      "ports": [
        "p1",
        "p2",
        "p3"
      ],
      "protocols": [
        "https",
        "ssh"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "ports": [
        "p1",
        "p2"
      ],
      "protocols": [
        "http"
      ]
    }
  ]
}
```

#### Example Request For Result Set

```http
GET /v1/data/opa/examples/allow_container?input=container:data.containers[container_index] HTTP/1.1
```

#### Example Response For Result Set

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "result": [
    [
      true,
      {
        "container_index": 0
      }
    ]
  ]
}
```

Result sets have the following schema:

```json
{
  "type": "array",
  "title": "Result Set",
  "description": "Set of results returned for a Data API query.",
  "items": {
    "type": "array",
    "minItems": 2,
    "maxItems": 2,
    "items": [
      {
        "title": "Value",
        "description": "The value of the document referred to by the Data API path."
      },
      {
        "type": "object",
        "title": "Bindings",
        "description": "The bindings of variables found in the Data API query input documents."
      }
    ]
  }
}
```

The example above assumes the following data and policy:

```json
{
  "containers": [
    {
        "Id": "a4288db7773ebb4f1b4d502712b87b241e3c28184cda6a1ad58f91ac6d89f052",
        "Created": "2016-11-21T03:13:14.288557666Z",
        "Path": "sh",
        "Args": [],
        "State": {
            "Status": "running",
            "Running": true,
            "Paused": false,
            "Restarting": false,
            "OOMKilled": false,
            "Dead": false,
            "Pid": 12127,
            "ExitCode": 0,
            "Error": "",
            "StartedAt": "2016-11-21T03:13:14.915355869Z",
            "FinishedAt": "0001-01-01T00:00:00Z"
        },
        "Image": "sha256:baa5d63471ead618ff91ddfacf1e2c81bf0612bfeb1daf00eb0843a41fbfade3",
        "ResolvConfPath": "/var/lib/docker/containers/a4288db7773ebb4f1b4d502712b87b241e3c28184cda6a1ad58f91ac6d89f052/resolv.conf",
        "HostnamePath": "/var/lib/docker/containers/a4288db7773ebb4f1b4d502712b87b241e3c28184cda6a1ad58f91ac6d89f052/hostname",
        "HostsPath": "/var/lib/docker/containers/a4288db7773ebb4f1b4d502712b87b241e3c28184cda6a1ad58f91ac6d89f052/hosts",
        "LogPath": "/var/lib/docker/containers/a4288db7773ebb4f1b4d502712b87b241e3c28184cda6a1ad58f91ac6d89f052/a4288db7773ebb4f1b4d502712b87b241e3c28184cda6a1ad58f91ac6d89f052-json.log",
        "Name": "/suspicious_brahmagupta",
        "RestartCount": 0,
        "Driver": "aufs",
        "MountLabel": "",
        "ProcessLabel": "",
        "AppArmorProfile": "",
        "ExecIDs": null,
        "HostConfig": {
            "Binds": null,
            "ContainerIDFile": "",
            "LogConfig": {
                "Type": "json-file",
                "Config": {}
            },
            "NetworkMode": "default",
            "PortBindings": {},
            "RestartPolicy": {
                "Name": "no",
                "MaximumRetryCount": 0
            },
            "AutoRemove": false,
            "VolumeDriver": "",
            "VolumesFrom": null,
            "CapAdd": null,
            "CapDrop": null,
            "Dns": [],
            "DnsOptions": [],
            "DnsSearch": [],
            "ExtraHosts": null,
            "GroupAdd": null,
            "IpcMode": "",
            "Cgroup": "",
            "Links": null,
            "OomScoreAdj": 0,
            "PidMode": "",
            "Privileged": false,
            "PublishAllPorts": false,
            "ReadonlyRootfs": false,
            "SecurityOpt": null,
            "UTSMode": "",
            "UsernsMode": "",
            "ShmSize": 67108864,
            "Runtime": "runc",
            "ConsoleSize": [
                0,
                0
            ],
            "Isolation": "",
            "CpuShares": 0,
            "Memory": 0,
            "CgroupParent": "",
            "BlkioWeight": 0,
            "BlkioWeightDevice": null,
            "BlkioDeviceReadBps": null,
            "BlkioDeviceWriteBps": null,
            "BlkioDeviceReadIOps": null,
            "BlkioDeviceWriteIOps": null,
            "CpuPeriod": 0,
            "CpuQuota": 0,
            "CpusetCpus": "",
            "CpusetMems": "",
            "Devices": [],
            "DiskQuota": 0,
            "KernelMemory": 0,
            "MemoryReservation": 0,
            "MemorySwap": 0,
            "MemorySwappiness": -1,
            "OomKillDisable": false,
            "PidsLimit": 0,
            "Ulimits": null,
            "CpuCount": 0,
            "CpuPercent": 0,
            "IOMaximumIOps": 0,
            "IOMaximumBandwidth": 0
        },
        "GraphDriver": {
            "Name": "aufs",
            "Data": null
        },
        "Mounts": [],
        "Config": {
            "Hostname": "a4288db7773e",
            "Domainname": "",
            "User": "",
            "AttachStdin": true,
            "AttachStdout": true,
            "AttachStderr": true,
            "Tty": true,
            "OpenStdin": true,
            "StdinOnce": true,
            "Env": [
                "no_proxy=*.local, 169.254/16",
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": [
                "sh"
            ],
            "Image": "alpine:latest",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": null,
            "OnBuild": null,
            "Labels": {}
        },
        "NetworkSettings": {
            "Bridge": "",
            "SandboxID": "2dadac1a8b18b7ae5658c4215637d998572c95bc0673bac2aceefbdd830d8860",
            "HairpinMode": false,
            "LinkLocalIPv6Address": "",
            "LinkLocalIPv6PrefixLen": 0,
            "Ports": {},
            "SandboxKey": "/var/run/docker/netns/2dadac1a8b18",
            "SecondaryIPAddresses": null,
            "SecondaryIPv6Addresses": null,
            "EndpointID": "fb1af875e2f0e7643224b6505a2c713748175689b8e8edb7cc1496efa8cdcafd",
            "Gateway": "172.17.0.1",
            "GlobalIPv6Address": "",
            "GlobalIPv6PrefixLen": 0,
            "IPAddress": "172.17.0.3",
            "IPPrefixLen": 16,
            "IPv6Gateway": "",
            "MacAddress": "02:42:ac:11:00:03",
            "Networks": {
                "bridge": {
                    "IPAMConfig": null,
                    "Links": null,
                    "Aliases": null,
                    "NetworkID": "ad1022afa0af59671f2b701ff8cbd4607de24740b59484acd4a740fac4ad26f9",
                    "EndpointID": "fb1af875e2f0e7643224b6505a2c713748175689b8e8edb7cc1496efa8cdcafd",
                    "Gateway": "172.17.0.1",
                    "IPAddress": "172.17.0.3",
                    "IPPrefixLen": 16,
                    "IPv6Gateway": "",
                    "GlobalIPv6Address": "",
                    "GlobalIPv6PrefixLen": 0,
                    "MacAddress": "02:42:ac:11:00:03"
                }
            }
        }
    },
    {
        "Id": "6887a5168d0e2324b8c9544c4b30321bce4952f12698491f268c17bbe77e5218",
        "Created": "2016-11-21T03:13:05.080079329Z",
        "Path": "sh",
        "Args": [],
        "State": {
            "Status": "running",
            "Running": true,
            "Paused": false,
            "Restarting": false,
            "OOMKilled": false,
            "Dead": false,
            "Pid": 12079,
            "ExitCode": 0,
            "Error": "",
            "StartedAt": "2016-11-21T03:13:05.718411107Z",
            "FinishedAt": "0001-01-01T00:00:00Z"
        },
        "Image": "sha256:baa5d63471ead618ff91ddfacf1e2c81bf0612bfeb1daf00eb0843a41fbfade3",
        "ResolvConfPath": "/var/lib/docker/containers/6887a5168d0e2324b8c9544c4b30321bce4952f12698491f268c17bbe77e5218/resolv.conf",
        "HostnamePath": "/var/lib/docker/containers/6887a5168d0e2324b8c9544c4b30321bce4952f12698491f268c17bbe77e5218/hostname",
        "HostsPath": "/var/lib/docker/containers/6887a5168d0e2324b8c9544c4b30321bce4952f12698491f268c17bbe77e5218/hosts",
        "LogPath": "/var/lib/docker/containers/6887a5168d0e2324b8c9544c4b30321bce4952f12698491f268c17bbe77e5218/6887a5168d0e2324b8c9544c4b30321bce4952f12698491f268c17bbe77e5218-json.log",
        "Name": "/fervent_almeida",
        "RestartCount": 0,
        "Driver": "aufs",
        "MountLabel": "",
        "ProcessLabel": "",
        "AppArmorProfile": "",
        "ExecIDs": null,
        "HostConfig": {
            "Binds": null,
            "ContainerIDFile": "",
            "LogConfig": {
                "Type": "json-file",
                "Config": {}
            },
            "NetworkMode": "default",
            "PortBindings": {},
            "RestartPolicy": {
                "Name": "no",
                "MaximumRetryCount": 0
            },
            "AutoRemove": false,
            "VolumeDriver": "",
            "VolumesFrom": null,
            "CapAdd": null,
            "CapDrop": null,
            "Dns": [],
            "DnsOptions": [],
            "DnsSearch": [],
            "ExtraHosts": null,
            "GroupAdd": null,
            "IpcMode": "",
            "Cgroup": "",
            "Links": null,
            "OomScoreAdj": 0,
            "PidMode": "",
            "Privileged": false,
            "PublishAllPorts": false,
            "ReadonlyRootfs": false,
            "SecurityOpt": [
                "seccomp:unconfined"
            ],
            "UTSMode": "",
            "UsernsMode": "",
            "ShmSize": 67108864,
            "Runtime": "runc",
            "ConsoleSize": [
                0,
                0
            ],
            "Isolation": "",
            "CpuShares": 0,
            "Memory": 0,
            "CgroupParent": "",
            "BlkioWeight": 0,
            "BlkioWeightDevice": null,
            "BlkioDeviceReadBps": null,
            "BlkioDeviceWriteBps": null,
            "BlkioDeviceReadIOps": null,
            "BlkioDeviceWriteIOps": null,
            "CpuPeriod": 0,
            "CpuQuota": 0,
            "CpusetCpus": "",
            "CpusetMems": "",
            "Devices": [],
            "DiskQuota": 0,
            "KernelMemory": 0,
            "MemoryReservation": 0,
            "MemorySwap": 0,
            "MemorySwappiness": -1,
            "OomKillDisable": false,
            "PidsLimit": 0,
            "Ulimits": null,
            "CpuCount": 0,
            "CpuPercent": 0,
            "IOMaximumIOps": 0,
            "IOMaximumBandwidth": 0
        },
        "GraphDriver": {
            "Name": "aufs",
            "Data": null
        },
        "Mounts": [],
        "Config": {
            "Hostname": "6887a5168d0e",
            "Domainname": "",
            "User": "",
            "AttachStdin": true,
            "AttachStdout": true,
            "AttachStderr": true,
            "Tty": true,
            "OpenStdin": true,
            "StdinOnce": true,
            "Env": [
                "no_proxy=*.local, 169.254/16",
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": [
                "sh"
            ],
            "Image": "alpine:latest",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": null,
            "OnBuild": null,
            "Labels": {}
        },
        "NetworkSettings": {
            "Bridge": "",
            "SandboxID": "580a6d3f8bebfb1647c8b34f7766dee1b86443752ef3e255d37517dfb35076ed",
            "HairpinMode": false,
            "LinkLocalIPv6Address": "",
            "LinkLocalIPv6PrefixLen": 0,
            "Ports": {},
            "SandboxKey": "/var/run/docker/netns/580a6d3f8beb",
            "SecondaryIPAddresses": null,
            "SecondaryIPv6Addresses": null,
            "EndpointID": "a4340ac5857a79310cc7eaa344d4ff6771fdf6372f72447b91928c038d9ada7d",
            "Gateway": "172.17.0.1",
            "GlobalIPv6Address": "",
            "GlobalIPv6PrefixLen": 0,
            "IPAddress": "172.17.0.2",
            "IPPrefixLen": 16,
            "IPv6Gateway": "",
            "MacAddress": "02:42:ac:11:00:02",
            "Networks": {
                "bridge": {
                    "IPAMConfig": null,
                    "Links": null,
                    "Aliases": null,
                    "NetworkID": "ad1022afa0af59671f2b701ff8cbd4607de24740b59484acd4a740fac4ad26f9",
                    "EndpointID": "a4340ac5857a79310cc7eaa344d4ff6771fdf6372f72447b91928c038d9ada7d",
                    "Gateway": "172.17.0.1",
                    "IPAddress": "172.17.0.2",
                    "IPPrefixLen": 16,
                    "IPv6Gateway": "",
                    "GlobalIPv6Address": "",
                    "GlobalIPv6PrefixLen": 0,
                    "MacAddress": "02:42:ac:11:00:02"
                }
            }
        }
    }]
}
```

```ruby
package opa.examples

import input.container

allow_container { not seccomp_unconfined }

seccomp_unconfined {
  container.HostConfig.SecurityOpt[_] = "seccomp:unconfined"
}
```

#### Query Parameters

- **input** - Provide an input document. Format is `[[<path>]:]<value>` where `<path>` is the import path of the input document. The parameter may be specified multiple times but each instance should specify a unique `<path>`. The `<path>` may be empty (in which case, the entire input will be set to the `<value>`). The `<value>` may be a reference to a document in OPA. If `<value>` contains variables the response will contain a set of results instead of a single document. In most cases, input should be provided using the POST method, see [Get a Document With Input](#get-a-document-with-input).
- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**, **truth**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if either:

- The query requires the input document and the caller does not provide it.
- The caller provides the input document but the query already defines it programmatically.

The server returns 200 if the path refers to an undefined document. In this
case, the response will not contain a `result` property.

### <a name="get-a-document-with-input"></a> Get a Document With Input

```
POST /v1/data/{path:.+}
Content-Type: application/json
```

```json
{
  "input": ...
}
```

Get a document that requires input.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

The request body contains an object that specifies a value for [The input Document](../../how-does-opa-work#the-input-document).

The examples below assume the following policy:

```ruby
package opa.examples

import input.example.flag

allow_request { flag = true }
```

#### Example Request

```http
POST /v1/data/opa/examples/allow_request HTTP/1.1
Content-Type: application/json
```

```json
{
  "input": {
    "example": {
      "flag": true
    }
  }
}
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": true
}
```

#### Example Request

```http
POST /v1/data/opa/examples/allow_request HTTP/1.1
Content-Type: application/json
```

```json
{
  "input": {
    "example": {
      "flag": false
    }
  }
}
```

#### Example Response

```http
HTTP/1.1 200 OK
```

```json
{}
```

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**, **truth**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if either:

1. The query requires an input document and the client did not supply one.
2. The query already defines an input document and the client did supply one.

The server returns 200 if the path refers to an undefined document. In this
case, the response will not contain a `result` property.

### Create or Overwrite a Document

```
PUT /v1/data/{path:.+}
Content-Type: application/json
```

Create or overwrite a document.

If the path does not refer to an existing document, the server will attempt to create all of the necessary containing documents. This behavior is similar in principle to the Unix command `mkdir -p`.

The server will respect the `If-None-Match` header if it is set to `*`. In this case, the server will not overwrite an existing document located at the path.

#### Example Request To Initialize Document With If-None-Match

```http
PUT /v1/data/us-west/servers HTTP/1.1
Content-Type: application/json
If-None-Match: *
```

```json
{}
```

#### Example Response If Document Already Exists

```http
HTTP/1.1 304 Not Modified
```

#### Example Response If Document Does Not Exist

```http
HTTP/1.1 204 No Content
```

#### Status Codes

- **204** - no content (success)
- **304** - not modified
- **404** - write conflict

If the path refers to a virtual document or a conflicting base document the server will respond with 404. A base document conflict will occur if the parent portion of the path refers to a non-object document.

### Patch a Document

```
PATCH /v1/data/{path:.+}
Content-Type: application/json-patch+json
```

Update a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

The server accepts updates encoded as JSON Patch operations. The message body of the request should contain a JSON encoded array containing one or more JSON Patch operations. Each operation specifies the operation type, path, and an optional value. For more information on JSON Patch, see [RFC 6902](https://tools.ietf.org/html/rfc6902).

#### Example Request

```http
PATCH /v1/data/servers HTTP/1.1
Content-Type: application/json-patch+json
```

```json
[
    {"op": "add",
     "path": "-",
     "value": {
         "id": "s5",
         "name": "job",
         "protocols": ["amqp"],
         "ports": ["p3"]
     }}
]
```

#### Example Response

```http
HTTP/1.1 204 No Content
```

#### Status Codes

- **204** - no content (success)
- **404** - not found
- **500** - server error

The effective path of the JSON Patch operation is obtained by joining the path portion of the URL with the path value from the operation(s) contained in the message body. In all cases, the parent of the effective path MUST refer to an existing document, otherwise the server returns 404. In the case of **remove** and **replace** operations, the effective path MUST refer to an existing document, otherwise the server returns 404.

## <a name="query-api"></a> Query API

### Execute a Query

```
GET /v1/query
```

Execute an ad-hoc query and return bindings for variables found in the query.

#### Example Request

```
GET /v1/query?q=data.servers[i].ports[_] = "p2"; data.servers[i].name = name HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "i": 3,
      "name": "dev"
    },
    {
      "i": 0,
      "name": "app"
    }
  ]
}
```

#### Query Parameters

- **q** - The ad-hoc query to execute. OPA will parse, compile, and execute the query represented by the parameter value. The value MUST be URL encoded.
- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**, **truth**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

## <a name="authentication"></a> Authentication

The API is secured via [HTTPS, Authentication, and
Authorization](/documentation/references/security).

### <a name="bearer-tokens"/> Bearer Tokens

When OPA is started with the ``--authentication=token`` command line flag,
clients MUST provide a Bearer token in the HTTP Authorization header:

```http
GET /v1/data/exempli-gratia HTTP/1.1
Authorization: Bearer my-secret-token
```

Bearer tokens must be represented with a valid HTTP header value character
sequence.

OPA will extract the Bearer token value (which is set to ``my-secret-token``
above) and provide it to the authorization component inside OPA that will (i)
validate the token and (ii) execute the authorization policy configured by the
admin.

## <a name="errors"></a> Errors

All of the API endpoints use standard HTTP status codes to indicate success or
failure of an API call. If an API call fails, the response will contain a JSON
encoded object that provides more detail. The `errors` and `location` fields are
optional:

```
{
  "code": "invalid_parameter",
  "message": "error(s) occurred while compiling module(s)",
  "errors": [
    {
      "code": "rego_unsafe_var_error",
      "message": "var x is unsafe",
      "location": {
        "file": "example",
        "row": 3,
        "col": 1
      }
    }
  ]
}
```

## <a name="explanations"></a> Explanations

OPA supports query explanations that describe (in detail) the steps taken to
produce query results.

Explanations can be requested for:

- [Data API](#data-api) GET queries
- [Query API](#query-api) queries

Explanations are requested by setting the `explain` query parameter to one of
the following values:

- **full** - returns a full query trace containing every step in the query evaluation process.
- **truth** - returns a partial query trace containing one path that leads to the overall query being successful.

### Trace Events

When the `explain` query parameter is set to **full** or **truth** , the
response contains an array of Trace Event objects.

Trace Event objects contain the following fields:

- **op** - identifies the kind of Trace Event. Values: **"Enter"**, **"Exit"**, **"Eval"**, **"Fail"**, **"Redo"**.
- **query_id** - uniquely identifies the query that the Trace Event was emitted for.
- **parent_id** - identifies the parent query.
- **type** - indicates the type of the **node** field. Values: **"expr"**, **"rule"**, **"body"**.
- **node** - contains the AST element associated with the evaluation step.
- **locals** - contains the term bindings from the query at the time when the Trace Event was emitted.

#### Query IDs

Queries often reference rules or contain comprehensions. In both cases, query
evaluation involves evaluation of one or more other queries, e.g., the body of
the rule or comprehension.

Trace Events from different queries can be distinguished by the **query_id**
field.

Trace Events from related queries can be identified by the **parent_id** field.

For example, if query A references a rule R, Trace Events emitted as part of
evaluating rule R's body will have the **parent_id** field set to query A's
**query_id**.

#### Types of Events

Each Trace Event represents a step in the query evaluation process. Trace Events
are emitted at the following points:

- **enter** - before a body or rule is evaluated.
- **exit** - after a body or rule has evaluated successfully.
- **eval** - before an expression is evaluated.
- **fail** - after an expression has evaluated to false.
- **redo** - before evaluation restarts from a body, rule, or expression.

By default, OPA searches for all sets of term bindings that make all expressions
in the query evaluate to true. Because there may be multiple answers, the search
can *restart* when OPA determines the query is true or false. When the search
restarts, a **Redo** Trace Event is emitted.

#### Example Trace Event

```json
{
  "op": "eval",
  "query_id": 20,
  "parent_id": 0,
  "type": "expr",
  "node": {
    "index": 1,
    "terms": [
      {
        "type": "var",
        "value": "eq"
      },
      {
        "type": "var",
        "value": "x"
      },
      {
        "type": "var",
        "value": "y"
      }
    ]
  },
  "locals": [
    {
      "key": {
        "type": "var",
        "value": "x"
      },
      "value": {
        "type": "string",
        "value": "hello"
      }
    }
  ]
}
```

## <a name="performance-metrics"></a> Performance Metrics

OPA can report detailed performance metrics at runtime. Currently, performance metrics can be requested on individual API calls and are returned inline with the API response. To enable performance metric collection on an API call, specify the `metrics=true` query parameter when executing the API call. Performance metrics are currently supported for the following APIs:

- Data API GET
- Data API POST
- Query API

For example:

```http
POST /v1/data/example?metrics=true HTTP/1.1
```

Response:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "metrics": {
    "timer_rego_query_compile_ns": 69994,
    "timer_rego_query_eval_ns": 48425,
    "timer_rego_query_parse_ns": 4096
  },
  "result": {
    "some_strings": [
      "hello",
      "world"
    ]
  }
}
```

### <a name="query-performance-metrics"></a> Query Performance Metrics

OPA currently supports the following query performance metrics:

- **timer_rego_query_parse_ns**: time taken (in nanonseconds) to parse the query.
- **timer_rego_query_compile_ns**: time taken (in nanonseconds) to compile the query.
- **timer_rego_query_eval_ns**: time taken (in nanonseconds) to evaluate the query.
- **timer_rego_module_parse_ns**: time taken (in nanoseconds) to parse the input policy module.
- **timer_rego_module_compile_ns**: time taken (in nanoseconds) to compile the loaded policy modules.

{% endcontentfor %}
