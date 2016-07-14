---
nav_id: MAIN_GET_OPA
layout: default

title: Get Open Policy Agent
---

{% contentfor header %}

# Get Open Policy Agent
{: .opa-header--minor-title}

OPA is available as binary releases for 64-bit Linux and Mac OS X or as Golang source code.
{: .opa-header--text}

  * [64-bit Linux](https://github.com/open-policy-agent/opa/releases/download/v0.1.0-rc3/opa_linux_amd64){: .opa-header--download-list--link}
  * [64-bit Mac OS X](https://github.com/open-policy-agent/opa/releases/download/v0.1.0-rc3/opa_darwin_amd64){: .opa-header--download-list--link}
  * [Go Source](https://github.com/open-policy-agent/opa/releases/download/v0.1.0-rc3/opa0.1rc3.src.tar.gz){: .opa-header--download-list--link}
  {: .opa-header--download-list}

{% endcontentfor %}

{% contentfor body %}

## 64-bit Linux

```shell
$ curl -L https://github.com/open-policy-agent/opa/releases/download/v0.1.0-rc3/opa_linux_amd64 > opa
$ chmod u+x opa
$ ./opa version
```

## 64-bit Mac OS X

```shell
$ curl -L https://github.com/open-policy-agent/opa/releases/download/v0.1.0-rc3/opa_darwin_amd64 > opa
$ chmod u+x opa
$ ./opa version
```

## Go Source

```shell
$ git clone https://github.com/open-policy-agent/opa.git $GOPATH/src/github.com/open-policy-agent/opa
$ cd $GOPATH/src/github.com/open-policy-agent/opa
$ make deps
$ make
$ ./opa version
```

{% endcontentfor %}
