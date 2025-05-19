---
title: Kubernetes Admission Control
software:
- kubernetes
labels:
  category: containers
  layer: orchestration
tutorials:
- https://www.openpolicyagent.org/docs/kubernetes-admission-control.html
- https://katacoda.com/austinheiman/scenarios/open-policy-agent-gatekeeper
code:
- https://github.com/open-policy-agent/kube-mgmt
- https://github.com/open-policy-agent/gatekeeper
inventors:
- styra
- microsoft
- google
videos:
- title: Securing Kubernetes With Admission Controllers
  speakers:
  - name: Dave Strebel
    organization: microsoft
  venue: Kubecon Seattle 2018
  link: https://sched.co/GrZQ
- title: Using OPA for Admission Control in Production
  speakers:
  - name: Zach Abrahamson
    organization: Capital One
  - name: Todd Ekenstam
    organization: Intuit
  venue: Kubecon Seattle 2018
  link: https://sched.co/Grbn
- title: Liz Rice Keynote
  speakers:
  - name: Liz Rice
    organization: AquaSecurity
  venue: Kubecon Seattle 2018
  link: https://youtu.be/McDzaTnUVWs?t=418
- title: Intro to Open Policy Agent Gatekeeper
  speakers:
  - name: Rita Zhang
    organization: microsoft
  - name: Max Smythe
    organization: google
  venue: Kubecon Barcelona 2019
  link: https://kccnceu19.sched.com/event/MPiM/intro-open-policy-agent-rita-zhang-microsoft-max-smythe-google
- title: Policy Enabled Kubernetes and CICD
  speakers:
  - name: Jimmy Ray
    organization: capitalone
  venue: OPA Summit at Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=vkvWZuqSk5M
- title: 'TripAdvisor: Building a Testing Framework for Integrating OPA into K8s'
  speakers:
  - name: Luke Massa
    organization: tripadvisor
  venue: OPA Summit at Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=X09c1eXvCFM
- title: Enforcing automatic mTLS with Linkerd and OPA Gatekeeper
  speakers:
  - name: Ivan Sim
    organization: buoyant
  - name: Rita Zhang
    organization: microsoft
  venue: Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=gMaGVHnvNfs
- title: Enforcing Service Mesh Structure using OPA Gatekeeper
  speakers:
  - name: Sandeep Parikh
    organization: google
  venue: Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=90RHTBinAFU
- title: 'TGIK: Exploring the Open Policy Agent'
  speakers:
  - name: Joe Beda
    organization: VMware
  link: https://www.youtube.com/watch?v=QU9BGPf0hBw
blogs:
- https://medium.com/@sbueringer/kubernetes-authorization-via-open-policy-agent-a9455d9d5ceb
- https://medium.com/@jimmy.ray/policy-enabled-kubernetes-with-open-policy-agent-3b612b3f0203
- https://blog.openpolicyagent.org/securing-the-kubernetes-api-with-open-policy-agent-ce93af0552c3
- https://itnext.io/kubernetes-authorization-via-open-policy-agent-a9455d9d5ceb
- https://medium.com/capital-one-tech/policy-enabled-kubernetes-with-open-policy-agent-3b612b3f0203
- https://blog.openshift.com/fine-grained-policy-enforcement-in-openshift-with-open-policy-agent/
docs_features:
  rest-api-integration:
    note: |
      The Kubernetes API server can be configured to use OPA as an
      admission controller. Creating a
      [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#validatingwebhookconfiguration)
      resource can be used to query OPA for policy decisions.
  kubernetes:
    note: |
      View a selection of projects and talks about integrating OPA with
      Kubernetes.
---
Kubernetes automates deployment, scaling, and management of containerized applications.  OPA provides fine-grained, context-aware authorization for which application component configuration.
