# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

package api

version := "0.10.0"

enforcement_points := {
    "mount_device": {"introducedVersion": "0.1.0", "default_results": {"allowed": false}},
    "mount_overlay": {"introducedVersion": "0.1.0", "default_results": {"allowed": false}},
    "create_container": {"introducedVersion": "0.1.0", "default_results": {"allowed": false, "env_list": null, "allow_stdio_access": false}},
    "unmount_device": {"introducedVersion": "0.2.0", "default_results": {"allowed": true}},
    "unmount_overlay": {"introducedVersion": "0.6.0", "default_results": {"allowed": true}},
    "exec_in_container": {"introducedVersion": "0.2.0", "default_results": {"allowed": true, "env_list": null}},
    "exec_external": {"introducedVersion": "0.3.0", "default_results": {"allowed": true, "env_list": null, "allow_stdio_access": false}},
    "shutdown_container": {"introducedVersion": "0.4.0", "default_results": {"allowed": true}},
    "signal_container_process": {"introducedVersion": "0.5.0", "default_results": {"allowed": true}},
    "plan9_mount": {"introducedVersion": "0.6.0", "default_results": {"allowed": true}},
    "plan9_unmount": {"introducedVersion": "0.6.0", "default_results": {"allowed": true}},
    "get_properties": {"introducedVersion": "0.7.0", "default_results": {"allowed": true}},
    "dump_stacks": {"introducedVersion": "0.7.0", "default_results": {"allowed": true}},
    "runtime_logging": {"introducedVersion": "0.8.0", "default_results": {"allowed": true}},
    "load_fragment": {"introducedVersion": "0.9.0", "default_results": {"allowed": false, "add_module": false}},
    "scratch_mount": {"introducedVersion": "0.10.0", "default_results": {"allowed": true}},
    "scratch_unmount": {"introducedVersion": "0.10.0", "default_results": {"allowed": true}},
}
