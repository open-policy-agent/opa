### Building and pushing a policy to an OCI registry

#### Using policy CLI

The [policy CLI](https://www.openpolicyregistry.io/docs/cli/download) tool can be easily used to build and push a policy bundle to a remote OCI registry using just two simple commands:
- `policy build <path_to_src> -t <org>/<repo>:<tag>`
- `policy push <registry>/<org>/<repo>:<tag>`
A full tutorial is available [here](https://www.openpolicyregistry.io/docs/tutorial)

#### Using OPA and ORAS CLIs

To build and push a policy bundle to a remote OCI registry with the [OPA CLI](https://www.openpolicyagent.org/docs/edge/cli/) and [ORAS CLI](https://oras.land/cli/) you can  use the following commands:

- `opa build <path_to_src>` will allow you to build a bundle tarball from your OPA policy and data files

Now that we have the tarball we will need to provide a config manifest to the ORAS CLI and the tarball itself: 
- `oras push <registry>/<org>/<repo>:<tag> --manifest-config <you_config_json>:application/vnd.oci.image.config.v1+json <the_tarball_obtained_from_opa_build>:application/vnd.oci.image.layer.v1.tar+gzip`

Using an empty(`{}`) `manifest-config` json file should be sufficient to be able to push and allow the OCI downloader to use the remote policy image. 

### OCI Downloader Debugging

Before starting to run a step-by-step debug process on the OCI downloader you will need a policy image pushed to a repository. To do this at the moment the easiest method is to use the [policy CLI](https://github.com/opcr-io/policy) to build your image and push it to a repository. You can use the public [opcr-io](https://opcr.io/) repository, [GHCR](https://ghcr.io) or any other OCI compatible repository. 

The easiest method to be able to do a step by step debugging of the OCI downloader is to start from the available `oci_download_test.go` file and replace the `fixture rest client` with an actual rest client component and use an adequate configuration. This client is then fed to the constructor of the `OCI downloader` and it will use the credentials to access the desired repository and download the image.

The `NewOCI` constructor receives the downloader configuration, rest client, the upstream image path (<server>/<organization>/<repository>:<tag>) and the local storage path as parameters.

**Example** of a rest client configuration: 
```
{
"name": "foo",
"url": "http://ghcr.io",
"credentials": {
  "bearer": {
     "token": "secret"
  }
 }
}
```
The `OCI Downloader` has the same behaviour as the default bundle downloader and the debugging process can follow the same pattern as seen in the test file. 

### OCI Downloader Limitations

The OCI Downloader uses the oras go library to pull images and the limitations of the current implementations are:
- it accepts only **one** layer per image that contains the bundle tarball
- it can download only the following application media types: 
    - `application/vnd.oci.image.layer.v1.tar+gzip`
    - `application/vnd.oci.image.manifest.v1+json`
    - `application/vnd.oci.image.config.v1+json`
- cannot create/push image using the OPA CLI 
- it can download only from OCI compatible registries 

### OCI Downloader TODOs

- Remove deprecated `ClearCache` function.
- Implement appropriate `SetCache` function for the OCI downloader. 