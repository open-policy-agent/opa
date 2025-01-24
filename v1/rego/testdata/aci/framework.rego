# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

package framework

import future.keywords.every
import future.keywords.in

version := "0.3.0"

device_mounted(target) {
    data.metadata.devices[target]
}

default deviceHash_ok := false

# test if a device hash exists as a layer in a policy container
deviceHash_ok {
    layer := data.policy.containers[_].layers[_]
    input.deviceHash == layer
}

# test if a device hash exists as a layer in a fragment container
deviceHash_ok {
    feed := data.metadata.issuers[_].feeds[_]
    some fragment in feed
    layer := fragment.containers[_].layers[_]
    input.deviceHash == layer
}

default mount_device := {"allowed": false}

mount_device := {"metadata": [addDevice], "allowed": true} {
    not device_mounted(input.target)
    deviceHash_ok
    addDevice := {
        "name": "devices",
        "action": "add",
        "key": input.target,
        "value": input.deviceHash,
    }
}

default unmount_device := {"allowed": false}

unmount_device := {"metadata": [removeDevice], "allowed": true} {
    device_mounted(input.unmountTarget)
    removeDevice := {
        "name": "devices",
        "action": "remove",
        "key": input.unmountTarget,
    }
}

layerPaths_ok(layers) {
    length := count(layers)
    count(input.layerPaths) == length
    every i, path in input.layerPaths {
        layers[(length - i) - 1] == data.metadata.devices[path]
    }
}

default overlay_exists := false

overlay_exists {
    data.metadata.matches[input.containerID]
}

overlay_mounted(target) {
    data.metadata.overlayTargets[target]
}

default candidate_containers := []

candidate_containers := containers {
    semver.compare(policy_framework_version, version) == 0

    policy_containers := [c | c := data.policy.containers[_]]
    fragment_containers := [c |
        feed := data.metadata.issuers[_].feeds[_]
        fragment := feed[_]
        c := fragment.containers[_]
    ]

    containers := array.concat(policy_containers, fragment_containers)
}

candidate_containers := containers {
    semver.compare(policy_framework_version, version) < 0

    policy_containers := apply_defaults("container", data.policy.containers, policy_framework_version)
    fragment_containers := [c |
        feed := data.metadata.issuers[_].feeds[_]
        fragment := feed[_]
        c := fragment.containers[_]
    ]

    containers := array.concat(policy_containers, fragment_containers)
}

default mount_overlay := {"allowed": false}

mount_overlay := {"metadata": [addMatches, addOverlayTarget], "allowed": true} {
    not overlay_exists

    containers := [container |
        container := candidate_containers[_]
        layerPaths_ok(container.layers)
    ]

    count(containers) > 0
    addMatches := {
        "name": "matches",
        "action": "add",
        "key": input.containerID,
        "value": containers,
    }

    addOverlayTarget := {
        "name": "overlayTargets",
        "action": "add",
        "key": input.target,
        "value": true,
    }
}

default unmount_overlay := {"allowed": false}

unmount_overlay := {"metadata": [removeOverlayTarget], "allowed": true} {
    overlay_mounted(input.unmountTarget)
    removeOverlayTarget := {
        "name": "overlayTargets",
        "action": "remove",
        "key": input.unmountTarget,
    }
}

command_ok(command) {
    count(input.argList) == count(command)
    every i, arg in input.argList {
        command[i] == arg
    }
}

env_ok(pattern, "string", value) {
    pattern == value
}

env_ok(pattern, "re2", value) {
    regex.match(pattern, value)
}

rule_ok(rule, env) {
    not rule.required
}

rule_ok(rule, env) {
    rule.required
    env_ok(rule.pattern, rule.strategy, env)
}

envList_ok(env_rules, envList) {
    every rule in env_rules {
        some env in envList
        rule_ok(rule, env)
    }

    every env in envList {
        some rule in env_rules
        env_ok(rule.pattern, rule.strategy, env)
    }
}

valid_envs_subset(env_rules) := envs {
    envs := {env |
        some env in input.envList
        some rule in env_rules
        env_ok(rule.pattern, rule.strategy, env)
    }
}

valid_envs_for_all(items) := envs {
    allow_environment_variable_dropping

    # for each item, find a subset of the environment rules
    # that are valid
    valid := [envs |
        some item in items
        envs := valid_envs_subset(item.env_rules)
    ]

    # we want to select the most specific matches, which in this
    # case consists of those matches which require dropping the
    # fewest environment variables (i.e. the longest lists)
    counts := [num_envs |
        envs := valid[_]
        num_envs := count(envs)
    ]
    max_count := max(counts)

    largest_env_sets := {envs |
        some i
        counts[i] == max_count
        envs := valid[i]
    }

    # if there is more than one set with the same size, we
    # can only proceed if they are all the same, so we verify
    # that the intersection is equal to the union. For a single
    # set this is trivially true.
    envs_i := intersection(largest_env_sets)
    envs_u := union(largest_env_sets)
    envs_i == envs_u
    envs := envs_i
}

valid_envs_for_all(items) := envs {
    not allow_environment_variable_dropping

    # no dropping allowed, so we just return the input
    envs := input.envList
}

workingDirectory_ok(working_dir) {
    input.workingDir == working_dir
}

privileged_ok(elevation_allowed) {
    not input.privileged
}

privileged_ok(elevation_allowed) {
    input.privileged
    input.privileged == elevation_allowed
}

noNewPrivileges_ok(no_new_privileges) {
    no_new_privileges
    input.noNewPrivileges
}

noNewPrivileges_ok(no_new_privileges) {
    no_new_privileges == false
}

idName_ok(pattern, "any", value) {
    true
}

idName_ok(pattern, "id", value) {
    pattern == value.id
}

idName_ok(pattern, "name", value) {
    pattern == value.name
}

idName_ok(pattern, "re2", value) {
    regex.match(pattern, value.name)
}

user_ok(user) {
    user.umask == input.umask
    idName_ok(user.user_idname.pattern, user.user_idname.strategy, input.user)
    every group in input.groups {
        some group_idname in user.group_idnames
        idName_ok(group_idname.pattern, group_idname.strategy, group)
    }
}

seccomp_ok(seccomp_profile_sha256) {
    input.seccompProfileSHA256 == seccomp_profile_sha256
}

default container_started := false

container_started {
    data.metadata.started[input.containerID]
}

default container_privileged := false

container_privileged {
    data.metadata.started[input.containerID].privileged
}

capsList_ok(allowed_caps_list, requested_caps_list) {
    count(allowed_caps_list) == count(requested_caps_list)

    every cap in requested_caps_list {
        some allowed in allowed_caps_list
        cap == allowed
    }

    every allowed in allowed_caps_list {
        some cap in requested_caps_list
        allowed == cap
    }
}

filter_capsList_by_allowed(allowed_caps_list, requested_caps_list) := caps {
    # find a subset of the capabilities that are valid
    caps := {cap |
        some cap in requested_caps_list
        some allowed in allowed_caps_list
        cap == allowed
    }
}

filter_capsList_for_single_container(allowed_caps) := caps {
    bounding := filter_capsList_by_allowed(allowed_caps.bounding, input.capabilities.bounding)
    effective := filter_capsList_by_allowed(allowed_caps.effective, input.capabilities.effective)
    inheritable := filter_capsList_by_allowed(allowed_caps.inheritable, input.capabilities.inheritable)
    permitted := filter_capsList_by_allowed(allowed_caps.permitted, input.capabilities.permitted)
    ambient := filter_capsList_by_allowed(allowed_caps.ambient, input.capabilities.ambient)

    caps := {
        "bounding": bounding,
        "effective": effective,
        "inheritable": inheritable,
        "permitted": permitted,
        "ambient": ambient
    }
}

largest_caps_sets_for_all(containers, privileged) := largest_caps_sets {
    filtered := [caps |
        container := containers[_]
        capabilities := get_capabilities(container, privileged)
        caps := filter_capsList_for_single_container(capabilities)
    ]

    # we want to select the most specific matches, which in this
    # case consists of those matches which require dropping the
    # fewest capabilities (i.e. the longest lists)
    counts := [num_caps |
        caps := filtered[_]
        num_caps := count(caps.bounding) + count(caps.effective) +
            count(caps.inheritable) + count(caps.permitted) +
            count(caps.ambient)
    ]
    max_count := max(counts)

    largest_caps_sets := [caps |
        some i
        counts[i] == max_count
        caps := filtered[i]
    ]
}

all_caps_sets_are_equal(sets) := caps {
    # if there is more than one set with the same size, we
    # can only proceed if they are all the same, so we verify
    # that the intersection is equal to the union. For a single
    # set this is trivially true.
    bounding_i := intersection({caps.bounding | caps := sets[_]})
    effective_i := intersection({caps.effective | caps := sets[_]})
    inheritable_i := intersection({caps.inheritable | caps := sets[_]})
    permitted_i := intersection({caps.permitted | caps := sets[_]})
    ambient_i := intersection({caps.ambient | caps := sets[_]})

    bounding_u := union({caps.bounding | caps := sets[_]})
    effective_u := union({caps.effective | caps := sets[_]})
    inheritable_u := union({caps.inheritable | caps := sets[_]})
    permitted_u := union({caps.permitted | caps := sets[_]})
    ambient_u := union({caps.ambient | caps := sets[_]})

    bounding_i == bounding_u
    effective_i == effective_u
    inheritable_i == inheritable_u
    permitted_i == permitted_u
    ambient_i == ambient_u

    caps := {
        "bounding": bounding_i,
        "effective": effective_i,
        "inheritable": inheritable_i,
        "permitted": permitted_i,
        "ambient": ambient_i,
    }
}

valid_caps_for_all(containers, privileged) := caps {
    allow_capability_dropping

    # find largest matching capabilities sets aka "the most specific"
    largest_caps_sets := largest_caps_sets_for_all(containers, privileged)

    # if there is more than one set with the same size, we
    # can only proceed if they are all the same
    caps := all_caps_sets_are_equal(largest_caps_sets)
}

valid_caps_for_all(containers, privileged) := caps {
    not allow_capability_dropping

    # no dropping allowed, so we just return the input
    caps := input.capabilities
}

caps_ok(allowed_caps, requested_caps) {
    capsList_ok(allowed_caps.bounding, requested_caps.bounding)
    capsList_ok(allowed_caps.effective, requested_caps.effective)
    capsList_ok(allowed_caps.inheritable, requested_caps.inheritable)
    capsList_ok(allowed_caps.permitted, requested_caps.permitted)
    capsList_ok(allowed_caps.ambient, requested_caps.ambient)
}

get_capabilities(container, privileged) := capabilities {
    container.capabilities != null
    capabilities := container.capabilities
}

default_privileged_capabilities := capabilities {
    caps := {cap | cap := data.defaultPrivilegedCapabilities[_]}
    capabilities := {
        "bounding": caps,
        "effective": caps,
        "inheritable": caps,
        "permitted": caps,
        "ambient": set(),
    }
}

get_capabilities(container, true) := capabilities {
    container.capabilities == null
    container.allow_elevated
    capabilities := default_privileged_capabilities
}

default_unprivileged_capabilities := capabilities {
    caps := {cap | cap := data.defaultUnprivilegedCapabilities[_]}
    capabilities := {
        "bounding": caps,
        "effective": caps,
        "inheritable": set(),
        "permitted": caps,
        "ambient": set(),
    }
}

get_capabilities(container, false) := capabilities {
    container.capabilities == null
    container.allow_elevated
    capabilities := default_unprivileged_capabilities
}

get_capabilities(container, privileged) := capabilities {
    container.capabilities == null
    not container.allow_elevated
    capabilities := default_unprivileged_capabilities
}

default create_container := {"allowed": false}

create_container := {"metadata": [updateMatches, addStarted],
                     "env_list": env_list,
                     "caps_list": caps_list,
                     "allow_stdio_access": allow_stdio_access,
                     "allowed": true} {
    not container_started

    # narrow the matches based upon command, working directory, and
    # mount list
    possible_after_initial_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        # NB any change to these narrowing conditions should be reflected in
        # the error handling, such that error messaging correctly reflects
        # the narrowing process.
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        privileged_ok(container.allow_elevated)
        workingDirectory_ok(container.working_dir)
        command_ok(container.command)
        mountList_ok(container.mounts, container.allow_elevated)
        seccomp_ok(container.seccomp_profile_sha256)
    ]

    count(possible_after_initial_containers) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_after_initial_containers)
    possible_after_env_containers := [container |
        container := possible_after_initial_containers[_]
        envList_ok(container.env_rules, env_list)
    ]

    count(possible_after_env_containers) > 0

    # check to see if the capabilities variables match, dropping
    # them if allowed (and necessary)
    caps_list := valid_caps_for_all(possible_after_env_containers, input.privileged)
    possible_after_caps_containers := [container |
        container := possible_after_env_containers[_]
        caps_ok(get_capabilities(container, input.privileged), caps_list)
    ]

    count(possible_after_caps_containers) > 0

    # set final container list
    containers := possible_after_caps_containers

    # we can't do narrowing based on allowing stdio access so at this point
    # every container from the policy that might match this create request
    # must have the same allow stdio value otherwise, we are in an undecidable
    # state
    allow_stdio_access := containers[0].allow_stdio_access
    every c in containers {
        c.allow_stdio_access == allow_stdio_access
    }

    updateMatches := {
        "name": "matches",
        "action": "update",
        "key": input.containerID,
        "value": containers,
    }

    addStarted := {
        "name": "started",
        "action": "add",
        "key": input.containerID,
        "value": {
            "privileged": input.privileged,
        },
    }
}

mountSource_ok(constraint, source) {
    startswith(constraint, data.sandboxPrefix)
    newConstraint := replace(constraint, data.sandboxPrefix, input.sandboxDir)
    regex.match(newConstraint, source)
}

mountSource_ok(constraint, source) {
    startswith(constraint, data.hugePagesPrefix)
    newConstraint := replace(constraint, data.hugePagesPrefix, input.hugePagesDir)
    regex.match(newConstraint, source)
}

mountSource_ok(constraint, source) {
    startswith(constraint, data.plan9Prefix)
    some target, containerID in data.metadata.p9mounts
    source == target
    input.containerID == containerID
}

mountSource_ok(constraint, source) {
    constraint == source
}

mountConstraint_ok(constraint, mount) {
    mount.type == constraint.type
    mountSource_ok(constraint.source, mount.source)
    mount.destination != ""
    mount.destination == constraint.destination

    # the following check is not required (as the following tests will prove this
    # condition as well), however it will check whether those more expensive
    # tests need to be performed.
    count(mount.options) == count(constraint.options)
    every option in mount.options {
        some constraintOption in constraint.options
        option == constraintOption
    }

    every option in constraint.options {
        some mountOption in mount.options
        option == mountOption
    }
}

mount_ok(mounts, allow_elevated, mount) {
    some constraint in mounts
    mountConstraint_ok(constraint, mount)
}

mount_ok(mounts, allow_elevated, mount) {
    some constraint in data.defaultMounts
    mountConstraint_ok(constraint, mount)
}

mount_ok(mounts, allow_elevated, mount) {
    allow_elevated
    some constraint in data.privilegedMounts
    mountConstraint_ok(constraint, mount)
}

mountList_ok(mounts, allow_elevated) {
    every mount in input.mounts {
        mount_ok(mounts, allow_elevated, mount)
    }
}

default exec_in_container := {"allowed": false}

exec_in_container := {"metadata": [updateMatches],
                      "env_list": env_list,
                      "caps_list": caps_list,
                      "allowed": true} {
    container_started

    # narrow our matches based upon the process requested
    possible_after_initial_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        # NB any change to these narrowing conditions should be reflected in
        # the error handling, such that error messaging correctly reflects
        # the narrowing process.
        workingDirectory_ok(container.working_dir)
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        some process in container.exec_processes
        command_ok(process.command)
    ]

    count(possible_after_initial_containers) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_after_initial_containers)
    possible_after_env_containers := [container |
        container := possible_after_initial_containers[_]
        envList_ok(container.env_rules, env_list)
    ]

    count(possible_after_env_containers) > 0

    # check to see if the capabilities variables match, dropping
    # them if allowed (and necessary)
    caps_list := valid_caps_for_all(possible_after_env_containers, container_privileged)
    possible_after_caps_containers := [container |
        container := possible_after_env_containers[_]
        caps_ok(get_capabilities(container, container_privileged), caps_list)
    ]

    count(possible_after_caps_containers) > 0

    # set final container list
    containers := possible_after_caps_containers

    updateMatches := {
        "name": "matches",
        "action": "update",
        "key": input.containerID,
        "value": containers,
    }
}

default shutdown_container := {"allowed": false}

shutdown_container := {"metadata": [remove], "allowed": true} {
    container_started
    remove := {
        "name": "matches",
        "action": "remove",
        "key": input.containerID,
    }
}

default signal_container_process := {"allowed": false}

signal_container_process := {"metadata": [updateMatches], "allowed": true} {
    container_started
    input.isInitProcess
    containers := [container |
        container := data.metadata.matches[input.containerID][_]
        signal_ok(container.signals)
    ]

    count(containers) > 0
    updateMatches := {
        "name": "matches",
        "action": "update",
        "key": input.containerID,
        "value": containers,
    }
}

signal_container_process := {"metadata": [updateMatches], "allowed": true} {
    container_started
    not input.isInitProcess
    containers := [container |
        container := data.metadata.matches[input.containerID][_]
        some process in container.exec_processes
        command_ok(process.command)
        signal_ok(process.signals)
    ]

    count(containers) > 0
    updateMatches := {
        "name": "matches",
        "action": "update",
        "key": input.containerID,
        "value": containers,
    }
}

signal_ok(signals) {
    some signal in signals
    input.signal == signal
}

plan9_mounted(target) {
    data.metadata.p9mounts[target]
}

default plan9_mount := {"allowed": false}

plan9_mount := {"metadata": [addPlan9Target], "allowed": true} {
    not plan9_mounted(input.target)
    some containerID, _ in data.metadata.matches
    pattern := concat("", [input.rootPrefix, "/", containerID, input.mountPathPrefix])
    regex.match(pattern, input.target)
    addPlan9Target := {
        "name": "p9mounts",
        "action": "add",
        "key": input.target,
        "value": containerID,
    }
}

default plan9_unmount := {"allowed": false}

plan9_unmount := {"metadata": [removePlan9Target], "allowed": true} {
    plan9_mounted(input.unmountTarget)
    removePlan9Target := {
        "name": "p9mounts",
        "action": "remove",
        "key": input.unmountTarget,
    }
}


default enforcement_point_info := {"available": false, "default_results": {"allow": false}, "unknown": true, "invalid": false, "version_missing": false}

enforcement_point_info := {"available": false, "default_results": {"allow": false}, "unknown": false, "invalid": false, "version_missing": true} {
    policy_api_version == null
}

enforcement_point_info := {"available": available, "default_results": default_results, "unknown": false, "invalid": false, "version_missing": false} {
    enforcement_point := data.api.enforcement_points[input.name]
    semver.compare(data.api.version, enforcement_point.introducedVersion) >= 0
    available := semver.compare(policy_api_version, enforcement_point.introducedVersion) >= 0
    default_results := enforcement_point.default_results
}

enforcement_point_info := {"available": false, "default_results": {"allow": false}, "unknown": false, "invalid": true, "version_missing": false} {
    enforcement_point := data.api.enforcement_points[input.name]
    semver.compare(data.api.version, enforcement_point.introducedVersion) < 0
}

default candidate_external_processes := []

candidate_external_processes := external_processes {
    semver.compare(policy_framework_version, version) == 0

    policy_external_processes := [e | e := data.policy.external_processes[_]]
    fragment_external_processes := [e |
        feed := data.metadata.issuers[_].feeds[_]
        fragment := feed[_]
        e := fragment.external_processes[_]
    ]

    external_processes := array.concat(policy_external_processes, fragment_external_processes)
}

candidate_external_processes := external_processes {
    semver.compare(policy_framework_version, version) < 0

    policy_external_processes := apply_defaults("external_process", data.policy.external_processes, policy_framework_version)
    fragment_external_processes := [e |
        feed := data.metadata.issuers[_].feeds[_]
        fragment := feed[_]
        e := fragment.external_processes[_]
    ]

    external_processes := array.concat(policy_external_processes, fragment_external_processes)
}

external_process_ok(process) {
    command_ok(process.command)
    envList_ok(process.env_rules, input.envList)
    workingDirectory_ok(process.working_dir)
}

default exec_external := {"allowed": false}

exec_external := {"allowed": true,
                  "allow_stdio_access": allow_stdio_access,
                  "env_list": env_list} {
    possible_processes := [process |
        process := candidate_external_processes[_]
        # NB any change to these narrowing conditions should be reflected in
        # the error handling, such that error messaging correctly reflects
        # the narrowing process.
        workingDirectory_ok(process.working_dir)
        command_ok(process.command)
    ]

    count(possible_processes) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_processes)
    processes := [process |
        process := possible_processes[_]
        envList_ok(process.env_rules, env_list)
    ]

    count(processes) > 0

    allow_stdio_access := processes[0].allow_stdio_access
    every p in processes {
        p.allow_stdio_access == allow_stdio_access
    }
}

default get_properties := {"allowed": false}

get_properties := {"allowed": true} {
    allow_properties_access
}

default dump_stacks := {"allowed": false}

dump_stacks := {"allowed": true} {
    allow_dump_stacks
}

default runtime_logging := {"allowed": false}

runtime_logging := {"allowed": true} {
    allow_runtime_logging
}

default fragment_containers := []

fragment_containers := data[input.namespace].containers

default fragment_fragments := []

fragment_fragments := data[input.namespace].fragments

default fragment_external_processes := []

fragment_external_processes := data[input.namespace].external_processes

apply_defaults(name, raw_values, framework_version) := values {
    semver.compare(framework_version, version) == 0
    values := raw_values
}

apply_defaults("container", raw_values, framework_version) := values {
    semver.compare(framework_version, version) < 0
    values := [checked |
        raw := raw_values[_]
        checked := check_container(raw, framework_version)
    ]
}

apply_defaults("external_process", raw_values, framework_version) := values {
    semver.compare(framework_version, version) < 0
    values := [checked |
        raw := raw_values[_]
        checked := check_external_process(raw, framework_version)
    ]
}

apply_defaults("fragment", raw_values, framework_version) := values {
    semver.compare(framework_version, version) < 0
    values := [checked |
        raw := raw_values[_]
        checked := check_fragment(raw, framework_version)
    ]
}

default fragment_framework_version := null
fragment_framework_version := data[input.namespace].framework_version

extract_fragment_includes(includes) := fragment {
    framework_version := fragment_framework_version
    objects := {
        "containers": apply_defaults("container", fragment_containers, framework_version),
        "fragments": apply_defaults("fragment", fragment_fragments, framework_version),
        "external_processes": apply_defaults("external_process", fragment_external_processes, framework_version)
    }

    fragment := {
        include: objects[include] | include := includes[_]
    }
}

issuer_exists(iss) {
    data.metadata.issuers[iss]
}

feed_exists(iss, feed) {
    data.metadata.issuers[iss].feeds[feed]
}

update_issuer(includes) := issuer {
    feed_exists(input.issuer, input.feed)
    old_issuer := data.metadata.issuers[input.issuer]
    old_fragments := old_issuer.feeds[input.feed]
    new_issuer := {"feeds": {input.feed: array.concat([extract_fragment_includes(includes)], old_fragments)}}

    issuer := object.union(old_issuer, new_issuer)
}

update_issuer(includes) := issuer {
    not feed_exists(input.issuer, input.feed)
    old_issuer := data.metadata.issuers[input.issuer]
    new_issuer := {"feeds": {input.feed: [extract_fragment_includes(includes)]}}

    issuer := object.union(old_issuer, new_issuer)
}

update_issuer(includes) := issuer {
    not issuer_exists(input.issuer)
    issuer := {"feeds": {input.feed: [extract_fragment_includes(includes)]}}
}

default candidate_fragments := []

candidate_fragments := fragments {
    semver.compare(policy_framework_version, version) == 0

    policy_fragments := [f | f := data.policy.fragments[_]]
    fragment_fragments := [f |
        feed := data.metadata.issuers[_].feeds[_]
        fragment := feed[_]
        f := fragment.fragments[_]
    ]

    fragments := array.concat(policy_fragments, fragment_fragments)
}

candidate_fragments := fragments {
    semver.compare(policy_framework_version, version) < 0

    policy_fragments := apply_defaults("fragment", data.policy.fragments, policy_framework_version)
    fragment_fragments := [f |
        feed := data.metadata.issuers[_].feeds[_]
        fragment := feed[_]
        f := fragment.fragments[_]
    ]

    fragments := array.concat(policy_fragments, fragment_fragments)
}

default load_fragment := {"allowed": false}

svn_ok(svn, minimum_svn) {
    # deprecated
    semver.is_valid(svn)
    semver.is_valid(minimum_svn)
    semver.compare(svn, minimum_svn) >= 0
}

svn_ok(svn, minimum_svn) {
    to_number(svn) >= to_number(minimum_svn)
}

fragment_ok(fragment) {
    input.issuer == fragment.issuer
    input.feed == fragment.feed
    svn_ok(data[input.namespace].svn, fragment.minimum_svn)
}

load_fragment := {"metadata": [updateIssuer], "add_module": add_module, "allowed": true} {
    some fragment in candidate_fragments
    fragment_ok(fragment)

    issuer := update_issuer(fragment.includes)
    updateIssuer := {
        "name": "issuers",
        "action": "update",
        "key": input.issuer,
        "value": issuer,
    }

    add_module := "namespace" in fragment.includes
}

default scratch_mount := {"allowed": false}

scratch_mounted(target) {
    data.metadata.scratch_mounts[target]
}

scratch_mount := {"metadata": [add_scratch_mount], "allowed": true} {
    not scratch_mounted(input.target)
    allow_unencrypted_scratch
    add_scratch_mount := {
        "name": "scratch_mounts",
        "action": "add",
        "key": input.target,
        "value": {"encrypted": input.encrypted},
    }
}

scratch_mount := {"metadata": [add_scratch_mount], "allowed": true} {
    not scratch_mounted(input.target)
    not allow_unencrypted_scratch
    input.encrypted
    add_scratch_mount := {
        "name": "scratch_mounts",
        "action": "add",
        "key": input.target,
        "value": {"encrypted": input.encrypted},
    }
}

default scratch_unmount := {"allowed": false}

scratch_unmount := {"metadata": [remove_scratch_mount], "allowed": true} {
    scratch_mounted(input.unmountTarget)
    remove_scratch_mount := {
        "name": "scratch_mounts",
        "action": "remove",
        "key": input.unmountTarget,
    }
}

reason := {
    "errors": errors,
    "error_objects": error_objects
}

################################################################
# Error messages
################################################################

errors["deviceHash not found"] {
    input.rule == "mount_device"
    not deviceHash_ok
}

errors["device already mounted at path"] {
    input.rule == "mount_device"
    device_mounted(input.target)
}

errors["no device at path to unmount"] {
    input.rule == "unmount_device"
    not device_mounted(input.unmountTarget)
}

errors["container already started"] {
    input.rule == "create_container"
    container_started
}

errors["container not started"] {
    input.rule in ["exec_in_container", "shutdown_container", "signal_container_process"]
    not container_started
}

errors["overlay has already been mounted"] {
    input.rule == "mount_overlay"
    overlay_exists
}

default overlay_matches := false

overlay_matches {
    some container in candidate_containers
    layerPaths_ok(container.layers)
}

errors["no overlay at path to unmount"] {
    input.rule == "unmount_overlay"
    not overlay_mounted(input.unmountTarget)
}

errors["no matching containers for overlay"] {
    input.rule == "mount_overlay"
    not overlay_matches
}

default privileged_matches := false

privileged_matches {
    input.rule == "create_container"
    some container in data.metadata.matches[input.containerID]
    privileged_ok(container.allow_elevated)
}

errors["privileged escalation not allowed"] {
    input.rule in ["create_container"]
    not privileged_matches
}

default command_matches := false

command_matches {
    input.rule == "create_container"
    some container in data.metadata.matches[input.containerID]
    command_ok(container.command)
}

command_matches {
    input.rule == "exec_in_container"
    some container in data.metadata.matches[input.containerID]
    some process in container.exec_processes
    command_ok(process.command)
}

command_matches {
    input.rule == "exec_external"
    some process in candidate_external_processes
    command_ok(process.command)
}

errors["invalid command"] {
    input.rule in ["create_container", "exec_in_container", "exec_external"]
    not command_matches
}

env_matches(env) {
    input.rule in ["create_container", "exec_in_container"]
    some container in data.metadata.matches[input.containerID]
    some rule in container.env_rules
    env_ok(rule.pattern, rule.strategy, env)
}

env_matches(env) {
    input.rule in ["exec_external"]
    some process in candidate_external_processes
    some rule in process.env_rules
    env_ok(rule.pattern, rule.strategy, env)
}

errors[envError] {
    input.rule in ["create_container", "exec_in_container", "exec_external"]
    bad_envs := [invalid |
        env := input.envList[_]
        not env_matches(env)
        parts := split(env, "=")
        invalid = parts[0]
    ]

    count(bad_envs) > 0
    envError := concat(" ", ["invalid env list:", concat(",", bad_envs)])
}

env_rule_matches(rule) {
    some env in input.envList
    env_ok(rule.pattern, rule.strategy, env)
}

errors["missing required environment variable"] {
    input.rule == "create_container"

    not container_started
    possible_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        privileged_ok(container.allow_elevated)
        workingDirectory_ok(container.working_dir)
        command_ok(container.command)
        mountList_ok(container.mounts, container.allow_elevated)
    ]

    count(possible_containers) > 0

    containers := [container |
        container := possible_containers[_]
        missing_rules := {invalid |
            invalid := {rule |
                rule := container.env_rules[_]
                rule.required
                not env_rule_matches(rule)
            }
            count(invalid) > 0
        }
        count(missing_rules) > 0
    ]

    count(containers) > 0
}

errors["missing required environment variable"] {
    input.rule == "exec_in_container"

    container_started
    possible_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        workingDirectory_ok(container.working_dir)
        some process in container.exec_processes
        command_ok(process.command)
    ]

    count(possible_containers) > 0

    containers := [container |
        container := possible_containers[_]
        missing_rules := {invalid |
            invalid := {rule |
                rule := container.env_rules[_]
                rule.required
                not env_rule_matches(rule)
            }
            count(invalid) > 0
        }
        count(missing_rules) > 0
    ]

    count(containers) > 0
}

errors["missing required environment variable"] {
    input.rule == "exec_external"

    possible_processes := [process |
        process := candidate_external_processes[_]
        workingDirectory_ok(process.working_dir)
        command_ok(process.command)
    ]

    count(possible_processes) > 0

    processes := [process |
        process := possible_processes[_]
        missing_rules := {invalid |
            invalid := {rule |
                rule := process.env_rules[_]
                rule.required
                not env_rule_matches(rule)
            }
            count(invalid) > 0
        }
        count(missing_rules) > 0
    ]

    count(processes) > 0
}

default workingDirectory_matches := false

workingDirectory_matches {
    input.rule in ["create_container", "exec_in_container"]
    some container in data.metadata.matches[input.containerID]
    workingDirectory_ok(container.working_dir)
}

workingDirectory_matches {
    input.rule == "exec_external"
    some process in candidate_external_processes
    workingDirectory_ok(process.working_dir)
}

errors["invalid working directory"] {
    input.rule in ["create_container", "exec_in_container", "exec_external"]
    not workingDirectory_matches
}

mount_matches(mount) {
    some container in data.metadata.matches[input.containerID]
    mount_ok(container.mounts, container.allow_elevated, mount)
}

errors[mountError] {
    input.rule == "create_container"
    bad_mounts := [mount.destination |
        mount := input.mounts[_]
        not mount_matches(mount)
    ]

    count(bad_mounts) > 0
    mountError := concat(" ", ["invalid mount list:", concat(",", bad_mounts)])
}

default signal_allowed := false

signal_allowed {
    some container in data.metadata.matches[input.containerID]
    signal_ok(container.signals)
}

signal_allowed {
    some container in data.metadata.matches[input.containerID]
    some process in container.exec_processes
    command_ok(process.command)
    signal_ok(process.signals)
}

errors["target isn't allowed to receive the signal"] {
    input.rule == "signal_container_process"
    not signal_allowed
}

errors["device already mounted at path"] {
    input.rule == "plan9_mount"
    plan9_mounted(input.target)
}

errors["no device at path to unmount"] {
    input.rule == "plan9_unmount"
    not plan9_mounted(input.unmountTarget)
}

default fragment_issuer_matches := false

fragment_issuer_matches {
    some fragment in candidate_fragments
    fragment.issuer == input.issuer
}

errors["invalid fragment issuer"] {
    input.rule == "load_fragment"
    not fragment_issuer_matches
}

default fragment_feed_matches := false

fragment_feed_matches {
    some fragment in candidate_fragments
    fragment.issuer == input.issuer
    fragment.feed == input.feed
}

fragment_feed_matches {
    input.feed in data.metadata.issuers[input.issuer]
}

errors["invalid fragment feed"] {
    input.rule == "load_fragment"
    fragment_issuer_matches
    not fragment_feed_matches
}

default fragment_version_is_valid := false

fragment_version_is_valid {
    some fragment in candidate_fragments
    fragment.issuer == input.issuer
    fragment.feed == input.feed
    svn_ok(data[input.namespace].svn, fragment.minimum_svn)
}

default svn_mismatch := false

svn_mismatch {
    some fragment in candidate_fragments
    fragment.issuer == input.issuer
    fragment.feed == input.feed
    to_number(data[input.namespace].svn)
    semver.is_valid(fragment.minimum_svn)
}

svn_mismatch {
    some fragment in candidate_fragments
    fragment.issuer == input.issuer
    fragment.feed == input.feed
    semver.is_valid(data[input.namespace].svn)
    to_number(fragment.minimum_svn)
}

errors["fragment svn is below the specified minimum"] {
    input.rule == "load_fragment"
    fragment_feed_matches
    not svn_mismatch
    not fragment_version_is_valid
}

errors["fragment svn and the specified minimum are different types"] {
    input.rule == "load_fragment"
    fragment_feed_matches
    svn_mismatch
}

errors["scratch already mounted at path"] {
    input.rule == "scratch_mount"
    scratch_mounted(input.target)
}

errors["unencrypted scratch not allowed"] {
    input.rule == "scratch_mount"
    not allow_unencrypted_scratch
    not input.encrypted
}

errors["no scratch at path to unmount"] {
    input.rule == "scratch_unmount"
    not scratch_mounted(input.unmountTarget)
}

errors[framework_version_error] {
    policy_framework_version == null
    framework_version_error := concat(" ", ["framework_version is missing. Current version:", version])
}

errors[framework_version_error] {
    semver.compare(policy_framework_version, version) > 0
    framework_version_error := concat(" ", ["framework_version is ahead of the current version:", policy_framework_version, "is greater than", version])
}

errors[fragment_framework_version_error] {
    input.namespace
    fragment_framework_version == null
    fragment_framework_version_error := concat(" ", ["fragment framework_version is missing. Current version:", version])
}

errors[fragment_framework_version_error] {
    input.namespace
    semver.compare(fragment_framework_version, version) > 0
    fragment_framework_version_error := concat(" ", ["fragment framework_version is ahead of the current version:", fragment_framework_version, "is greater than", version])
}

errors["containers only distinguishable by allow_stdio_access"] {
    input.rule == "create_container"

    not container_started
    possible_after_initial_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        privileged_ok(container.allow_elevated)
        workingDirectory_ok(container.working_dir)
        command_ok(container.command)
        mountList_ok(container.mounts, container.allow_elevated)
        seccomp_ok(container.seccomp_profile_sha256)
    ]

    count(possible_after_initial_containers) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_after_initial_containers)
    possible_after_env_containers := [container |
        container := possible_after_initial_containers[_]
        envList_ok(container.env_rules, env_list)
    ]

    count(possible_after_env_containers) > 0

    # check to see if the capabilities variables match, dropping
    # them if allowed (and necessary)
    caps_list := valid_caps_for_all(possible_after_env_containers, input.privileged)
    possible_after_caps_containers := [container |
        container := possible_after_env_containers[_]
        caps_ok(get_capabilities(container, input.privileged), caps_list)
    ]

    count(possible_after_caps_containers) > 0

    # set final container list
    containers := possible_after_caps_containers

    allow_stdio_access := containers[0].allow_stdio_access
    some c in containers
    c.allow_stdio_access != allow_stdio_access
}

errors["external processes only distinguishable by allow_stdio_access"] {
    input.rule == "exec_external"

    possible_processes := [process |
        process := candidate_external_processes[_]
        workingDirectory_ok(process.working_dir)
        command_ok(process.command)
    ]

    count(possible_processes) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_processes)
    processes := [process |
        process := possible_processes[_]
        envList_ok(process.env_rules, env_list)
    ]

    count(processes) > 0

    allow_stdio_access := processes[0].allow_stdio_access
    some p in processes
    p.allow_stdio_access != allow_stdio_access
}


default noNewPrivileges_matches := false

noNewPrivileges_matches {
    input.rule == "create_container"
    some container in data.metadata.matches[input.containerID]
    noNewPrivileges_ok(container.no_new_privileges)
}

noNewPrivileges_matches {
    input.rule == "exec_in_container"
    some container in data.metadata.matches[input.containerID]
    some process in container.exec_processes
    command_ok(process.command)
    workingDirectory_ok(process.working_dir)
    noNewPrivileges_ok(process.no_new_privileges)
}

errors["invalid noNewPrivileges"] {
    input.rule in ["create_container", "exec_in_container"]
    not noNewPrivileges_matches
}

default user_matches := false

user_matches {
    input.rule == "create_container"
    some container in data.metadata.matches[input.containerID]
    user_ok(container.user)
}

user_matches {
    input.rule == "exec_in_container"
    some container in data.metadata.matches[input.containerID]
    some process in container.exec_processes
    command_ok(process.command)
    workingDirectory_ok(process.working_dir)
    user_ok(process.user)
}

errors["invalid user"] {
    input.rule in ["create_container", "exec_in_container"]
    not user_matches
}

errors["capabilities don't match"] {
    input.rule == "create_container"

    not container_started

    possible_after_initial_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        privileged_ok(container.allow_elevated)
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        workingDirectory_ok(container.working_dir)
        command_ok(container.command)
        mountList_ok(container.mounts, container.allow_elevated)
        seccomp_ok(container.seccomp_profile_sha256)
    ]

    count(possible_after_initial_containers) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_after_initial_containers)
    possible_after_env_containers := [container |
        container := possible_after_initial_containers[_]
        envList_ok(container.env_rules, env_list)
    ]

    count(possible_after_env_containers) > 0

    # check to see if the capabilities variables match, dropping
    # them if allowed (and necessary)
    caps_list := valid_caps_for_all(possible_after_env_containers, input.privileged)
    possible_after_caps_containers := [container |
        container := possible_after_env_containers[_]
        caps_ok(get_capabilities(container, input.privileged), caps_list)
    ]

    count(possible_after_caps_containers) == 0
}

errors["capabilities don't match"] {
    input.rule == "exec_in_container"

    container_started

    possible_after_initial_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        workingDirectory_ok(container.working_dir)
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        some process in container.exec_processes
        command_ok(process.command)
    ]

    count(possible_after_initial_containers) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_after_initial_containers)
    possible_after_env_containers := [container |
        container := possible_after_initial_containers[_]
        envList_ok(container.env_rules, env_list)
    ]

    count(possible_after_env_containers) > 0

    # check to see if the capabilities variables match, dropping
    # them if allowed (and necessary)
    caps_list := valid_caps_for_all(possible_after_env_containers, container_privileged)
    possible_after_caps_containers := [container |
        container := possible_after_env_containers[_]
        caps_ok(get_capabilities(container, container_privileged), caps_list)
    ]

    count(possible_after_caps_containers) == 0
}

# covers exec_in_container as well. it shouldn't be possible to ever get
# an exec_in_container as it "inherits" capabilities rules from create_container
errors["containers only distinguishable by capabilties"] {
    input.rule == "create_container"

    allow_capability_dropping
    not container_started

    # narrow the matches based upon command, working directory, and
    # mount list
    possible_after_initial_containers := [container |
        container := data.metadata.matches[input.containerID][_]
        # NB any change to these narrowing conditions should be reflected in
        # the error handling, such that error messaging correctly reflects
        # the narrowing process.
        noNewPrivileges_ok(container.no_new_privileges)
        user_ok(container.user)
        privileged_ok(container.allow_elevated)
        workingDirectory_ok(container.working_dir)
        command_ok(container.command)
        mountList_ok(container.mounts, container.allow_elevated)
    ]

    count(possible_after_initial_containers) > 0

    # check to see if the environment variables match, dropping
    # them if allowed (and necessary)
    env_list := valid_envs_for_all(possible_after_initial_containers)
    possible_after_env_containers := [container |
        container := possible_after_initial_containers[_]
        envList_ok(container.env_rules, env_list)
    ]

    count(possible_after_env_containers) > 0

    largest := largest_caps_sets_for_all(possible_after_env_containers, input.privileged)
    not all_caps_sets_are_equal(largest)
}

default seccomp_matches := false

seccomp_matches {
    input.rule == "create_container"
    some container in data.metadata.matches[input.containerID]
    seccomp_ok(container.seccomp_profile_sha256)
}

errors["invalid seccomp"] {
    input.rule == "create_container"
    not seccomp_matches
}

default error_objects := null

error_objects := containers {
    input.rule == "create_container"
    containers := data.metadata.matches[input.containerID]
}

error_objects := processes {
    input.rule == "exec_in_container"
    processes := [process |
        container := data.metadata.matches[input.containerID][_]
        process := container.exec_processes[_]
    ]
}

error_objects := processes {
    input.rule == "exec_external"
    processes := candidate_external_processes
}

error_objects := fragments {
    input.rule == "load_fragment"
    fragments := candidate_fragments
}


################################################################################
# Logic for providing backwards compatibility for framework data objects
################################################################################


check_container(raw_container, framework_version) := container {
    semver.compare(framework_version, version) == 0
    container := raw_container
}

check_container(raw_container, framework_version) := container {
    semver.compare(framework_version, version) < 0
    container := {
        # Base fields
        "command": raw_container.command,
        "env_rules": raw_container.env_rules,
        "layers": raw_container.layers,
        "mounts": raw_container.mounts,
        "allow_elevated": raw_container.allow_elevated,
        "working_dir": raw_container.working_dir,
        "exec_processes": raw_container.exec_processes,
        "signals": raw_container.signals,
        "allow_stdio_access": raw_container.allow_stdio_access,
        # Additional fields need to have default logic applied
        "no_new_privileges": check_no_new_privileges(raw_container, framework_version),
        "user": check_user(raw_container, framework_version),
        "capabilities": check_capabilities(raw_container, framework_version),
        "seccomp_profile_sha256": check_seccomp_profile_sha256(raw_container, framework_version),
    }
}

check_no_new_privileges(raw_container, framework_version) := no_new_privileges {
    semver.compare(framework_version, "0.2.0") >= 0
    no_new_privileges := raw_container.no_new_privileges
}

check_no_new_privileges(raw_container, framework_version) := no_new_privileges {
    semver.compare(framework_version, "0.2.0") < 0
    no_new_privileges := false
}

check_user(raw_container, framework_version) := user {
    semver.compare(framework_version, "0.2.1") >= 0
    user := raw_container.user
}

check_user(raw_container, framework_version) := user {
    semver.compare(framework_version, "0.2.1") < 0
    user := {
        "umask": "0022",
        "user_idname": {
            "pattern": "",
            "strategy": "any"
        },
        "group_idnames": [
            {
                "pattern": "",
                "strategy": "any"
            }
        ]
    }
}

check_capabilities(raw_container, framework_version) := capabilities {
    semver.compare(framework_version, "0.2.2") >= 0
    capabilities := raw_container.capabilities
}

check_capabilities(raw_container, framework_version) := capabilities {
    semver.compare(framework_version, "0.2.2") < 0
    # we cannot determine a reasonable default at the time this is called,
    # which is either during `mount_overlay` or `load_fragment`, and so
    # we set it to `null`, which indicates that the capabilities should
    # be determined dynamically when needed.
    capabilities := null
}

check_seccomp_profile_sha256(raw_container, framework_version) := seccomp_profile_sha256 {
    semver.compare(framework_version, "0.2.3") >= 0
    seccomp_profile_sha256 := raw_container.seccomp_profile_sha256
}

check_seccomp_profile_sha256(raw_container, framework_version) := seccomp_profile_sha256 {
    semver.compare(framework_version, "0.2.3") < 0
    seccomp_profile_sha256 := ""
}

check_external_process(raw_process, framework_version) := process {
    semver.compare(framework_version, version) == 0
    process := raw_process
}

check_external_process(raw_process, framework_version) := process {
    semver.compare(framework_version, version) < 0
    process := {
        # Base fields
        "command": raw_process.command,
        "env_rules": raw_process.env_rules,
        "working_dir": raw_process.working_dir,
        "allow_stdio_access": raw_process.allow_stdio_access,
        # Additional fields need to have default logic applied
    }
}

check_fragment(raw_fragment, framework_version) := fragment {
    semver.compare(framework_version, version) == 0
    fragment := raw_fragment
}

check_fragment(raw_fragment, framework_version) := fragment {
    semver.compare(framework_version, version) < 0
    fragment := {
        # Base fields
        "issuer": raw_fragment.issuer,
        "feed": raw_fragment.feed,
        "minimum_svn": raw_fragment.minimum_svn,
        "includes": raw_fragment.includes,
        # Additional fields need to have default logic applied
    }
}

# base policy-level flags
allow_properties_access := data.policy.allow_properties_access
allow_dump_stacks := data.policy.allow_dump_stacks
allow_runtime_logging := data.policy.allow_runtime_logging
allow_environment_variable_dropping := data.policy.allow_environment_variable_dropping
allow_unencrypted_scratch := data.policy.allow_unencrypted_scratch

# all flags not in the base set need to have default logic applied

default allow_capability_dropping := false

allow_capability_dropping := flag {
    semver.compare(policy_framework_version, "0.2.2") >= 0
    flag := data.policy.allow_capability_dropping
}

default policy_framework_version := null
default policy_api_version := null

policy_framework_version := data.policy.framework_version
policy_api_version := data.policy.api_version

# deprecated
policy_framework_version := data.policy.framework_svn
policy_api_version := data.policy.api_svn
fragment_framework_version := data[input.namespace].framework_svn
