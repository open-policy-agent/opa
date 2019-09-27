# Integration Index (Total 19)
## Index of Enforcement Points by Layer of the Stack

**orchestration**
* <a href="#kubernetes-validating-admission">Kubernetes Admission Control</a>
* <a href="#clair-datasource">Kubernetes Admission Control using Vulnerability Scanning</a>
* <a href="#terraform">Terraform Authorization</a>

**network**
* <a href="#kong-authorization">API Gateway Authorization with Kong</a>
* <a href="#springsecurity-api">Authorization for Java Spring Security</a>
* <a href="#envoy-authorization">Container Network Authorization with Envoy</a>
* <a href="#istio-authorization-mixer">Container Network Authorization with Istio (as part of Mixer)</a>
* <a href="#istio-authorization-edge">Container Network Authorization with Istio (at the Edge)</a>
* <a href="#iptables">IPTables</a>

**server**
* <a href="#docker-machine">Docker controls via OPA Policies</a>
* <a href="#linux-pam">SSH and Sudo Authorization with Linux</a>

**data**
* <a href="#ceph">Ceph Object Storage Authorization</a>
* <a href="#elasticsearch-datafiltering">Elasticsearch Data Filtering</a>
* <a href="#kafka-authorization">Kafka Topic Authorization</a>
* <a href="#minio">Minio API Authorization</a>
* <a href="#sql-datafiltering">SQL Database Data Filtering</a>

**cicd**
* <a href="#spinnaker-pipeline">Spinnaker Pipeline Policy Enforcment</a>

**application**
* <a href="#cloudflare-worker">Cloudflare Worker Enforcement of OPA Policies Using WASM</a>
## Index of Tools Powered by OPA
* <a href="#terraform">Conftest -- Configuration checking</a>
## Integration Details

###  <a name="kubernetes-validating-admission">Kubernetes Admission Control</a>
Kubernetes automates deployment, scaling, and management of containerized applications.  OPA provides fine-grained, context-aware authorization for which application component configuration.

**Software**: <a href="https://kubernetes.io">Kubernetes</a>

**Inventors**: <a href="https://styra.com">Styra</a>, <a href="https://microsoft.com">Microsoft</a>, <a href="https://google.com">Google</a>

**Tutorials**
* <a href="https://www.openpolicyagent.org/docs/kubernetes-admission-control.html">https://www.openpolicyagent.org/docs/kubernetes-admission-control.html</a>

**Code**
* <a href="https://github.com/open-policy-agent/kube-mgmt">https://github.com/open-policy-agent/kube-mgmt</a>
* <a href="https://github.com/open-policy-agent/gatekeeper">https://github.com/open-policy-agent/gatekeeper</a>

**Videos**
* <a href="https://sched.co/GrZQ">Securing Kubernetes With Admission Controllers</a> by Dave Strebel from <a href="https://microsoft.com">Microsoft</a> at Kubecon Seattle 2018
* <a href="https://sched.co/Grbn">Using OPA for Admission Control in Production</a> by Zach Abrahamson from Capital One, Todd Ekenstam from Intuit at Kubecon Seattle 2018
* <a href="https://youtu.be/McDzaTnUVWs?t=418">Liz Rice Keynote</a> by Liz Rice from AquaSecurity at Kubecon Seattle 2018
* <a href="https://kccnceu19.sched.com/event/MPiM/intro-open-policy-agent-rita-zhang-microsoft-max-smythe-google">Intro to Open Policy Agent Gatekeeper</a> by Rita Zhang from <a href="https://microsoft.com">Microsoft</a>, Max Smythe from <a href="https://google.com">Google</a> at Kubecon Barcelona 2019

**Blogs**
* <a href="https://medium.com/@sbueringer/kubernetes-authorization-via-open-policy-agent-a9455d9d5ceb">https://medium.com/@sbueringer/kubernetes-authorization-via-open-policy-agent-a9455d9d5ceb</a>
* <a href="https://medium.com/@jimmy.ray/policy-enabled-kubernetes-with-open-policy-agent-3b612b3f0203">https://medium.com/@jimmy.ray/policy-enabled-kubernetes-with-open-policy-agent-3b612b3f0203</a>
* <a href="https://blog.openpolicyagent.org/securing-the-kubernetes-api-with-open-policy-agent-ce93af0552c3">https://blog.openpolicyagent.org/securing-the-kubernetes-api-with-open-policy-agent-ce93af0552c3</a>

###  <a name="envoy-authorization">Container Network Authorization with Envoy</a>
Envoy is a networking abstraction for cloud-native applications. OPA hooks into Envoy’s external authorization filter to provide fine-grained, context-aware authorization for network or HTTP requests.

**Software**: <a href="https://envoyproxy.io">Envoy</a>

**Inventors**: <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://github.com/tsandall/minimal-opa-envoy-example/blob/master/README.md">https://github.com/tsandall/minimal-opa-envoy-example/blob/master/README.md</a>
* <a href="https://www.openpolicyagent.org/docs/latest/envoy-authorization/">https://www.openpolicyagent.org/docs/latest/envoy-authorization/</a>

**Code**
* <a href="https://github.com/open-policy-agent/opa-istio-plugin">https://github.com/open-policy-agent/opa-istio-plugin</a>
* <a href="https://github.com/tsandall/minimal-opa-envoy-example">https://github.com/tsandall/minimal-opa-envoy-example</a>

**Blogs**
* <a href="https://blog.openpolicyagent.org/envoy-external-authorization-with-opa-578213ed567c">https://blog.openpolicyagent.org/envoy-external-authorization-with-opa-578213ed567c</a>

###  <a name="istio-authorization-edge">Container Network Authorization with Istio (at the Edge)</a>
Istio is a networking abstraction for cloud-native applications that uses Envoy at the edge. OPA hooks into Envoy’s external authorization filter to provide fine-grained, context-aware authorization for network or HTTP requests.

**Software**: <a href="https://istio.io">Istio</a>, <a href="https://envoyproxy.io">Envoy</a>, spire

**Inventors**: <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://github.com/open-policy-agent/opa-istio-plugin/blob/master/README.md">https://github.com/open-policy-agent/opa-istio-plugin/blob/master/README.md</a>

**Code**
* <a href="https://github.com/open-policy-agent/opa-istio-plugin">https://github.com/open-policy-agent/opa-istio-plugin</a>
* <a href="https://github.com/tsandall/minimal-opa-envoy-example">https://github.com/tsandall/minimal-opa-envoy-example</a>
* <a href="https://github.com/open-policy-agent/opa-envoy-spire-ext-authz">https://github.com/open-policy-agent/opa-envoy-spire-ext-authz</a>

**Blogs**
* <a href="https://blog.openpolicyagent.org/envoy-external-authorization-with-opa-578213ed567c">https://blog.openpolicyagent.org/envoy-external-authorization-with-opa-578213ed567c</a>

###  <a name="istio-authorization-mixer">Container Network Authorization with Istio (as part of Mixer)</a>
Istio is a networking abstraction for cloud-native applications. In this Istio integration OPA hooks into the centralized Mixer component of Istio, to provide fine-grained, context-aware authorization for network or HTTP requests.

**Software**: <a href="https://istio.io">Istio</a>

**Inventors**: <a href="https://google.com">Google</a>

**Tutorials**
* <a href="https://istio.io/docs/reference/config/policy-and-telemetry/adapters/opa/">https://istio.io/docs/reference/config/policy-and-telemetry/adapters/opa/</a>

**Code**
* <a href="https://github.com/istio/istio/tree/master/mixer/adapter/opa">https://github.com/istio/istio/tree/master/mixer/adapter/opa</a>

###  <a name="kong-authorization">API Gateway Authorization with Kong</a>
Kong is a microservice API Gateway.  OPA provides fine-grained, context-aware control over the requests that Kong receives.

**Software**: <a href="https://konghq.com/">Kong</a>

**Inventors**: <a href="http://travelnest.com">TravelNest</a>

**Code**
* <a href="https://github.com/TravelNest/kong-authorization-opa">https://github.com/TravelNest/kong-authorization-opa</a>

###  <a name="linux-pam">SSH and Sudo Authorization with Linux</a>
Host-level access controls are an important part of every organization's security strategy. OPA provides fine-grained, context-aware controls for SSH and sudo using Linux-PAM.

**Software**: <a href="http://www.linux-pam.org/">Linux PAM</a>

**Inventors**: <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://www.openpolicyagent.org/docs/ssh-and-sudo-authorization.html">https://www.openpolicyagent.org/docs/ssh-and-sudo-authorization.html</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/pam_authz">https://github.com/open-policy-agent/contrib/tree/master/pam_authz</a>

###  <a name="kafka-authorization">Kafka Topic Authorization</a>
Apache Kafka is a high-performance distributed streaming platform deployed by thousands of companies.  OPA provides fine-grained, context-aware access control of which users can read/write which Kafka topics to enforce important requirements around confidentiality and integrity.

**Software**: <a href="https://kafka.apache.org/">Kafka</a>

**Inventors**: <a href="https://www.ticketmaster.com/">TicketMaster</a>, <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://www.openpolicyagent.org/docs/kafka-authorization.html">https://www.openpolicyagent.org/docs/kafka-authorization.html</a>

**Code**
* <a href="https://github.com/llofberg/kafka-authorizer-opa">https://github.com/llofberg/kafka-authorizer-opa</a>
* <a href="https://github.com/open-policy-agent/contrib/tree/master/kafka_authorizer">https://github.com/open-policy-agent/contrib/tree/master/kafka_authorizer</a>

###  <a name="ceph">Ceph Object Storage Authorization</a>
Ceph is a highly scalable distributed storage solution that uniquely delivers object, block, and file storage in one unified system.  OPA provides fine-grained, context-aware authorization of the information stored within Ceph.

**Software**: <a href="https://ceph.io/">Ceph</a>

**Inventors**: <a href="https://styra.com">Styra</a>, <a href="https://www.redhat.com">RedHat</a>

**Tutorials**
* <a href="https://docs.ceph.com/docs/master/radosgw/opa/">https://docs.ceph.com/docs/master/radosgw/opa/</a>
* <a href="https://www.katacoda.com/styra/scenarios/opa-ceph">https://www.katacoda.com/styra/scenarios/opa-ceph</a>

**Videos**
* <a href="https://www.youtube.com/watch?v=9m4FymEvOqM&feature=share">https://www.youtube.com/watch?v=9m4FymEvOqM&feature=share</a>

###  <a name="minio">Minio API Authorization</a>
Minio is an open source, on-premise object database compatible with the Amazon S3 API.  This integration lets OPA enforce policies on Minio's API.

**Software**: <a href="https://min.io/">Minio</a>

**Inventors**: <a href="https://min.io/">Minio</a>, <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://github.com/minio/minio/blob/master/docs/sts/opa.md">https://github.com/minio/minio/blob/master/docs/sts/opa.md</a>

###  <a name="terraform">Terraform Authorization</a>
Terraform lets you describe the infrastructure you want and automatically creates, deletes, and modifies your existing infrastructure to match. OPA makes it possible to write policies that test the changes Terraform is about to make before it makes them.

**Software**: <a href="https://www.terraform.io/">Terraform</a>, <a href="https://aws.com">Amazon Public Cloud</a>, <a href="https://cloud.google.com/">Google Public Cloud</a>, <a href="http://azure.microsoft.com/">Microsoft Public Cloud</a>

**Inventors**: <a href="https://www.medallia.com/">Medallia</a>, <a href="https://styra.com">Styra</a>, <a href="https://www.docker.com/">Docker</a>, <a href="https://snyk.io/">Snyk</a>

**Tutorials**
* <a href="https://www.openpolicyagent.org/docs/terraform.html">https://www.openpolicyagent.org/docs/terraform.html</a>
* <a href="https://github.com/instrumenta/conftest/blob/master/README.md">https://github.com/instrumenta/conftest/blob/master/README.md</a>

**Code**
* <a href="https://github.com/instrumenta/conftest">https://github.com/instrumenta/conftest</a>

###  <a name="iptables">IPTables</a>
IPTables is a useful tool available to Linux kernel for filtering network packets. OPA makes it possible to manage IPTables rules using context-aware policy.

**Software**: Linux

**Inventors**: Urvil Patel at <a href="https://summerofcode.withgoogle.com/">Google Summer of Code</a>, <a href="https://www.cisco.com/">Cisco</a>, <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://github.com/open-policy-agent/contrib/blob/master/opa-iptables/docs/tutorial.md">https://github.com/open-policy-agent/contrib/blob/master/opa-iptables/docs/tutorial.md</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/opa-iptables">https://github.com/open-policy-agent/contrib/tree/master/opa-iptables</a>

###  <a name="springsecurity-api">Authorization for Java Spring Security</a>
Spring Security provides a framework for securing Java applications.  This integration provides a simple implementation of an AccessDecisionVoter for Spring Security that uses OPA for making API authorization decisions.

**Software**: <a href="https://spring.io/projects/spring-security">Spring Security</a>

**Inventors**: <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://github.com/open-policy-agent/contrib/blob/master/spring_authz/README.md">https://github.com/open-policy-agent/contrib/blob/master/spring_authz/README.md</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/spring_authz">https://github.com/open-policy-agent/contrib/tree/master/spring_authz</a>

###  <a name="spinnaker-pipeline">Spinnaker Pipeline Policy Enforcment</a>
Spinnaker is a  Continuous Delivery and Deployment tool started by Netflix.  OPA lets you configure policies that dictate what kinds of Spinnaker pipelines developers can create.

**Software**: <a href="https://www.spinnaker.io/">Spinnaker</a>

**Inventors**: <a href="https://www.armory.io/">Armory</a>

**Tutorials**
* <a href="https://docs.armory.io/spinnaker/policy_engine/">https://docs.armory.io/spinnaker/policy_engine/</a>

###  <a name="elasticsearch-datafiltering">Elasticsearch Data Filtering</a>
Elasticsearch is a distributed, open source search and analytics engine.  This OPA integration lets an elasticsearch client construct queries so that the data returned by elasticsearch obeys OPA-defined policies.

**Software**: <a href="https://www.elastic.co/">Elastic Search</a>

**Inventors**: <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://github.com/open-policy-agent/contrib/blob/master/data_filter_elasticsearch/README.md">https://github.com/open-policy-agent/contrib/blob/master/data_filter_elasticsearch/README.md</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/data_filter_elasticsearch">https://github.com/open-policy-agent/contrib/tree/master/data_filter_elasticsearch</a>

###  <a name="sql-datafiltering">SQL Database Data Filtering</a>
This integration enables the client of a SQL database to enhance a SQL query so that the results obey an OPA-defined policy.

**Software**: <a href="https://www.sqlite.org/index.html">SQLite</a>

**Inventors**: <a href="https://styra.com">Styra</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/data_filter_example">https://github.com/open-policy-agent/contrib/tree/master/data_filter_example</a>

**Blogs**
* <a href="https://blog.openpolicyagent.org/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4">https://blog.openpolicyagent.org/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4</a>

###  <a name="clair-datasource">Kubernetes Admission Control using Vulnerability Scanning</a>
Admission control policies in Kubernetes can be augmented with vulnerability scanning results to make more informed decisions.  This integration demonstrates how to integrate Clair with OPA and run it as an admission controller.

**Software**: <a href="https://kubernetes.io">Kubernetes</a>, <a href="https://github.com/coreos/clair">Clair</a>

**Tutorials**
* <a href="https://github.com/open-policy-agent/contrib/blob/master/image_enforcer/README.md">https://github.com/open-policy-agent/contrib/blob/master/image_enforcer/README.md</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/image_enforcer">https://github.com/open-policy-agent/contrib/tree/master/image_enforcer</a>

###  <a name="cloudflare-worker">Cloudflare Worker Enforcement of OPA Policies Using WASM</a>
Cloudflare Workers are a serverless platform that supports WASM.  This integration uses OPA's WASM compiler to generate code enforced at the edge of Cloudflare's network.

**Software**: <a href="https://www.cloudflare.com">Cloudflare</a>

**Tutorials**
* <a href="https://github.com/open-policy-agent/contrib/blob/master/wasm/cloudflare-worker/README.md">https://github.com/open-policy-agent/contrib/blob/master/wasm/cloudflare-worker/README.md</a>

**Code**
* <a href="https://github.com/open-policy-agent/contrib/tree/master/wasm/cloudflare-worker">https://github.com/open-policy-agent/contrib/tree/master/wasm/cloudflare-worker</a>

###  <a name="docker-machine">Docker controls via OPA Policies</a>
Docker's out of the box authorization model is all or nothing.  This integration demonstrates how to use OPA's context-aware policies to exert fine-grained control over Docker.

**Software**: <a href="https://www.docker.com/">Docker</a>

**Inventors**: <a href="https://styra.com">Styra</a>

**Tutorials**
* <a href="https://www.openpolicyagent.org/docs/latest/docker-authorization/">https://www.openpolicyagent.org/docs/latest/docker-authorization/</a>

**Code**
* <a href="https://github.com/open-policy-agent/opa-docker-authz">https://github.com/open-policy-agent/opa-docker-authz</a>

###  <a name="conftest">Conftest -- Configuration checking</a>
Conftest is a utility built on top of OPA to help you write tests against structured configuration data.

**Software**: CUE, Kustomize, <a href="https://www.terraform.io/">Terraform</a>, Serverless Framework, AWS SAM Framework, INI, TOML, Dockerfile, HCL2

**Code**
* <a href="https://github.com/instrumenta/conftest">https://github.com/instrumenta/conftest</a>
