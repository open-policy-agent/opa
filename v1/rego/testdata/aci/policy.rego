# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

package policy

api_version := "0.10.0"
framework_version := "0.3.0"

fragments := [
    {"issuer": "did:web:contoso.com", "feed": "contoso.azurecr.io/infra", "minimum_svn": "1", "includes": ["containers"]},
]
containers := [
    {
        "command": ["rustc","--help"],
        "env_rules": [{"pattern": `PATH=/usr/local/cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`, "strategy": "string", "required": true},{"pattern": `RUSTUP_HOME=/usr/local/rustup`, "strategy": "string", "required": true},{"pattern": `CARGO_HOME=/usr/local/cargo`, "strategy": "string", "required": true},{"pattern": `RUST_VERSION=1.52.1`, "strategy": "string", "required": true},{"pattern": `TERM=xterm`, "strategy": "string", "required": false},{"pattern": `PREFIX_.+=.+`, "strategy": "re2", "required": false}],
        "layers": ["fe84c9d5bfddd07a2624d00333cf13c1a9c941f3a261f13ead44fc6a93bc0e7a","4dedae42847c704da891a28c25d32201a1ae440bce2aecccfa8e6f03b97a6a6c","41d64cdeb347bf236b4c13b7403b633ff11f1cf94dbc7cf881a44d6da88c5156","eb36921e1f82af46dfe248ef8f1b3afb6a5230a64181d960d10237a08cd73c79","e769d7487cc314d3ee748a4440805317c19262c7acd2fdbdb0d47d2e4613a15c","1b80f120dbd88e4355d6241b519c3e25290215c469516b49dece9cf07175a766"],
        "mounts": [{"destination": "/container/path/one", "options": ["rbind","rshared","rw"], "source": "sandbox:///host/path/one", "type": "bind"},{"destination": "/container/path/two", "options": ["rbind","rshared","ro"], "source": "sandbox:///host/path/two", "type": "bind"}],
        "exec_processes": [{"command": ["top"], "signals": []}],
        "signals": [],
        "user": {
            "user_idname": {"pattern": ``, "strategy": "any"},
            "group_idnames": [{"pattern": ``, "strategy": "any"}],
            "umask": "0022"
        },
        "capabilities": {
            "bounding": ["CAP_SYS_ADMIN"],
            "effective": ["CAP_SYS_ADMIN"],
            "inheritable": ["CAP_SYS_ADMIN"],
            "permitted": ["CAP_SYS_ADMIN"],
            "ambient": ["CAP_SYS_ADMIN"],
        },
        "seccomp_profile_sha256": "",
        "allow_elevated": true,
        "working_dir": "/home/user",
        "allow_stdio_access": false,
        "no_new_privileges": true,
    },
    {
        "command": ["/pause"],
        "env_rules": [{"pattern": `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`, "strategy": "string", "required": true},{"pattern": `TERM=xterm`, "strategy": "string", "required": false}],
        "layers": ["16b514057a06ad665f92c02863aca074fd5976c755d26bff16365299169e8415"],
        "mounts": [],
        "exec_processes": [],
        "signals": [],
        "user": {
            "user_idname": {"pattern": ``, "strategy": "any"},
            "group_idnames": [{"pattern": ``, "strategy": "any"}],
            "umask": "0022"
        },
        "capabilities": null,
        "seccomp_profile_sha256": "",
        "allow_elevated": false,
        "working_dir": "/",
        "allow_stdio_access": false,
        "no_new_privileges": true,
    },
]
external_processes := [
    {"command": ["bash"], "env_rules": [{"pattern": `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`, "strategy": "string", "required": true}], "working_dir": "/", "allow_stdio_access": false},
]
allow_properties_access := false
allow_dump_stacks := false
allow_runtime_logging := false
allow_environment_variable_dropping := false
allow_unencrypted_scratch := false
allow_capability_dropping := true


mount_device := data.framework.mount_device
unmount_device := data.framework.unmount_device
mount_overlay := data.framework.mount_overlay
unmount_overlay := data.framework.unmount_overlay
create_container := data.framework.create_container
exec_in_container := data.framework.exec_in_container
exec_external := data.framework.exec_external
shutdown_container := data.framework.shutdown_container
signal_container_process := data.framework.signal_container_process
plan9_mount := data.framework.plan9_mount
plan9_unmount := data.framework.plan9_unmount
get_properties := data.framework.get_properties
dump_stacks := data.framework.dump_stacks
runtime_logging := data.framework.runtime_logging
load_fragment := data.framework.load_fragment
scratch_mount := data.framework.scratch_mount
scratch_unmount := data.framework.scratch_unmount
reason := {
    "errors": data.framework.errors,
    "error_objects": data.framework.error_objects,
}
