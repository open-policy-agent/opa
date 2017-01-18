package opa.examples

import input.container

allow_container :-
    not seccomp_unconfined

seccomp_unconfined :-
    container.HostConfig.SecurityOpt[_] = "seccomp:unconfined"
