---
nav_id: MAIN_GET_OPA
layout: default

title: Get Open Policy Agent
---

{% contentfor header %}

# Get Open Policy Agent
{: .opa-header--minor-title}

The binary releases for 64-bit Linux and Mac are available for download here. For other releases of OPA see the [GitHub Releases](https://github.com/open-policy-agent/opa/releases) page.
{: .opa-header--text}

  * [64-bit Linux](https://github.com/open-policy-agent/opa/releases/download/v0.3.0/opa_linux_amd64){: .opa-header--download-list--link}
  * [64-bit Mac OS X](https://github.com/open-policy-agent/opa/releases/download/v0.3.0/opa_darwin_amd64){: .opa-header--download-list--link}
  * [Go Source](https://github.com/open-policy-agent/opa/archive/v0.3.0.tar.gz){: .opa-header--download-list--link}
  {: .opa-header--download-list}

{% endcontentfor %}

{% contentfor body %}

## 64-bit Linux

```shell
$ curl -L https://github.com/open-policy-agent/opa/releases/download/v0.3.0/opa_linux_amd64 > opa
$ chmod u+x opa
$ ./opa version
```

## 64-bit Mac OS X

```shell
$ curl -L https://github.com/open-policy-agent/opa/releases/download/v0.3.0/opa_darwin_amd64 > opa
$ chmod u+x opa
$ ./opa version
```

## Docker Image (64-bit Linux)
```shell
$ docker run openpolicyagent/opa:0.3.0 version
```

{% endcontentfor %}
