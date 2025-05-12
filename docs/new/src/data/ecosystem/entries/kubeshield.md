---
title: KubeShield
subtitle: Secure Kubernetes using eBPF & Open Policy Agent
software:
- linux
- kubernetes
- ebpf
labels:
  layer: application
  category: filtering
code:
- https://github.com/kubeshield/bpf-opa-demo
blogs:
- https://blog.byte.builders/post/bpf-opa/
docs_features:
  kubernetes:
    note: |
      KubeShield implements runtime policy for containers in a Kubernetes
      cluster using eBPF. Follow the
      [tutorial here](https://github.com/kubeshield/bpf-opa-demo#usage)
      to get up and running.
---
Ensure runtime security in any linux machine by combining Extended Berkeley Packet Filter(eBPF) and Open Policy Agent.
