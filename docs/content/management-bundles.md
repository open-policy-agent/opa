---
title: "Bundle API Implementations"
kind: management
weight: 2
---

The Bundle API is simple. Most HTTP servers capable of serving static files will do. While not strictly required in all deployments, it is also good if the implementation supports:

* HTTP caching using the [ETag header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/ETag). This keeps OPA from having to download a bundle unless the bundle's content have changes.
* Authentication. When exposing a bundle at a remote endpoint, it is often desirable to protect the data by requiring all requests to the endpoint to be authenticated.

This document lists some of the more common HTTP servers suitable as bundle servers, along with instructions for how to set them up as such.

## Amazon S3

### OPA Bundle Support

| Feature | Supported |
|---------|-----------|
| Caching headers | Yes |
| Authentication methods | [AWS Signature](https://www.openpolicyagent.org/docs/latest/configuration/#aws-signature) |

### Setup Instructions

1. Search for "S3" and on the "Buckets" page, click "Create bucket".
2. Fill in the form according to your preferences (name, region, etc).
3. Either choose "Block all public access" for internal systems, or unmark the checkbox for that to allow external (authenticated) requests.
4. You can now upload your bundle to the bucket. If you try to download it right away you'll notice that by default you're unauthorized to do so.
5. To allow anyone to read the bundle, click on it and select "Make public" from the "Object actions" dropdown menu. If not, proceed to configure authentication.

### Authentication

Authentication can be configured to either use the credentials of a service account stored in the environment, or to use credentials fetched from the AWS metadata API. The latter is only available from services running inside of AWS (on EC2 or ECS).

Both methods are going to need a policy for either the service account or the IAM role, so when that is mentioned in the steps for either method you may refer to the example below.

**Example IAM policy**
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::my-example-opa-bucket"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject"
            ],
            "Resource": [
                "arn:aws:s3:::my-example-opa-bucket/*"
            ]
        }
    ]
}
```

#### Environment Credentials

1. Go to the "IAM" section of the AWS console. Choose "Users" and "Create new user". Select a name for the user, and the "Programmatic access" option.
2. On the following "Permissions" page, choose "Attach existing policies directly" and then press "Create policy". Select the JSON tab and paste a policy like the example shown above, replacing `my-example-opa-bucket` with the name of your bucket.
3. Once the policy has been created, it can be assigned to the user. With the user having been created, make sure to note down the AWS access key ID and the AWS secret access key, as they will be the credentials used for authentication.

#### Metadata Credentials

1. Go to the "IAM" section of the AWS console. Choose "Roles" and "Create role". For type, select "AWS service" and for use case, choose EC2, or wherever you'll be running OPA.
2. On the following "Permissions" page, choose "Create policy". Select the JSON tab and paste a policy like the example shown above, replacing `my-example-opa-bucket` with the name of your bucket.
3. Once the policy has been created, it can be assigned to the role.
4. With the role created, go to the EC2 instance view. Select an instance where OPA will run and select "Actions" -> "Security" -> "Modify IAM role". Select the role created in previous steps.

#### Testing Authentication

Testing authentication and access directly with something like Curl is non-trivial, mainly due to the steps required in crafting an [AWS Signature](https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-authenticating-requests.html). Recommended approach is to instead use the [AWS CLI tools]([command line tools](https://aws.amazon.com/cli/)) (see "Upload Bundle" below).

### Upload Bundle

Bundle uploads to S3 are easily facilitated using the `aws` command in the [AWS CLI tools](https://aws.amazon.com/cli/).

```shell
aws --profile=opa-service-account s3 cp bundle.tar.gz s3://my-example-opa-bucket/
```

### Example OPA Configuration

#### Environment Credentials

With the environment variables `AWS_REGION`, `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` set, the following configuration will extract the credentials from the [environment](https://www.openpolicyagent.org/docs/latest/configuration/#using-static-environment-credentials).

```yaml
services:
  s3:
    url: https://my-example-opa-bucket.s3.eu-north-1.amazonaws.com
    credentials:
      s3_signing:
        environment_credentials: {}

bundles:
  authz:
    service: s3
    resource: bundle.tar.gz
```

#### Metadata Credentials

In order for this to work it is required that the permissions you created in the "Authentication" steps above are embedded in an IAM Role, which is then assigned to the EC2 instance hosting OPA.

```yaml
services:
  s3:
    url: https://my-example-opa-bucket.s3.eu-north-1.amazonaws.com
    credentials:
      s3_signing:
        metadata_credentials:
          aws_region: eu-north-1
          iam_role: my-opa-bucket-access-role

bundles:
  authz:
    service: s3
    resource: bundle.tar.gz
```

## Google Cloud Storage

### OPA Bundle Support

| Feature | Supported |
|---------|-----------|
| Caching headers | Yes |
| Authentication methods | [GCP Metadata Token](https://www.openpolicyagent.org/docs/latest/configuration/#gcp-metadata-token) <br>[OAuth2 JWT Bearer Grant Type](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-jwt-bearer-grant-type) |

### Setup Instructions

1. In the left pane menu, choose "Cloud Storage". Click "New bucket".
2. Fill in the form according to your preferences (name, region, availability, etc).
3. Once the bucket is created, you can press "Upload" to upload a test bundle. Clicking this will provide a link to the bundle which you can use in your OPA configuration.
4. At this stage you can either choose to make the bucket public (by clicking "Permissions") or to configure a service account for authenticated access.

### Authentication

#### GCP Metadata Token Authentication

If your instance of OPA runs inside GCP, you'll be able to authenticate using GCP metadata tokens. These tokens by default carry all the permissions granted to the default service account, so you might still want to create a dedicated service account for this purpose (see [JWT Bearer Grant Type](#jwt-bearer-grant-type) below).

#### JWT Bearer Grant Type

Use this for [authenticating](https://cloud.google.com/storage/docs/authentication) _external_ clients, i.e. OPAs running outside the GCP environment.

1. Search for "credentials" in the top search box and choose "Credentials - APIs and Services".
2. Click "Create Credentials" followed by "Service Account."
3. Fill in a name for the account and proceed to select roles.
4. Choose "Storage Object Viewer" for read access and "Storage Object Creator" for write access (if scripted uploads is desired).
5. Click the newly created service account and then the "Keys" tab. Press "Add Key" and either "Create new" or upload an existing one.
6. If creating new, choose to download the private key in JSON format (not P12).
7. Open the JSON file just downloaded and copy the PEM encoded value of the `private_key` attribute. This is the key you'll use for your OPA configuration.

#### Testing Authentication

The easiest way of testing GCP metadata token or JWT bearer grant type authentication is simply to set up OPA with config for these and run the server.

### Upload Bundle

Uploading a bundle is trivial with the `gsutil` command included with the [Google Cloud SDK](https://cloud.google.com/sdk/docs/quickstart).

```shell
gsutil cp bundle.tar.gz gs://<bucket-name>/
```

### Example OPA Configuration

#### GCP Metadata Token Authentication

```yaml
services:
  gcs:
    url: https://storage.googleapis.com/storage/v1/b/opa-docs/o
    credentials:
      gcp_metadata:
        scopes:
          - https://www.googleapis.com/auth/devstorage.read_only

bundles:
  authz:
    service: gcs
    resource: 'bundle.tar.gz?alt=media'
```

#### Google Cloud Storage Bundle and JWT Bearer Authentication

```yaml
services:
  gcp:
    url: ${BUNDLE_SERVICE_URL}
    credentials:
      oauth2:
        grant_type: jwt_bearer
        signing_key: jwt_signing_key # references the key in `keys` below
        scopes:
        - https://www.googleapis.com/auth/devstorage.read_only
        additional_claims:
          aud: https://oauth2.googleapis.com/token
          iss: opa-client@my-account.iam.gserviceaccount.com

bundles:
  authz:
    service: gcp
    resource: bundles/http/example/authz.tar.gz

keys:
  jwt_signing_key:
    algorithm: RS256
    private_key: ${BUNDLE_SERVICE_SIGNING_KEY}
```

## Azure Blob Storage

### OPA Bundle Support

| Feature | Supported |
|---------|-----------|
| Caching headers | Yes |
| Authentication methods | [OAuth2 Client Credentials](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-client-credentials), <br> [OAuth2 Client Credentials JWT authentication](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-client-credentials-jwt-authentication) |

Note that for the time being, the [Shared Key or Shared Access Signature (SAS)](https://docs.microsoft.com/en-us/rest/api/storageservices/authorize-requests-to-azure-storage) options are [not supported](https://github.com/open-policy-agent/opa/issues/2964).

### Setup Instructions

1. Any type of storage in Azure is grouped in Storage Accounts. If you have one already, skip to step 3.
2. From the Azure console, select "Storage Accounts" followed by "New". Fill in the form (name, region, etc) according to your preferences. One thing to note when selecting "account kind", make sure to pick the Storage V2 (general purpose v2) option and not the legacy BlobStorage kind.
3. With the storage account deployed, press "Go to resource" to create a new storage resource.
4. Select "Containers" and press the plus sign to create a new storage container.
5. Name your container and select access level. Choose "Private" to require authentication, or "Blob" to allow unauthenticated read access.
6. Press "upload" and select the bundle from your local filesystem.
7. Clicking the filename should bring up a properties window where the public URL to the bundle is included.

### Authentication

1. Go to Azure Active Directory.
2. In the left menu, click "App Registrations" followed by "New Registration". Name your app (client) amd leave the other options be. Click "Register".
3. Click "Certificates and Secrets". Either create a secret to be used for [OAuth2 Client Credentials](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-client-credentials) or upload a certificate for [OAuth2 Client Credentials JWT authentication](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-client-credentials-jwt-authentication).
4. In the menu to the left, click "API permissions". Click "Add a permission". Choose "Azure Storage" and check the "user_impersonation" checkbox.
5. Click "Add admin consent for Default Directory". Answer Yes on the followup question.
6. Navigate back to your storage account. Click "Access Control (IAM)". Click "Add role assignments".
7. Select the "Storage Blob Data Contributor" role. Leave "Assign access to" as "User, group or service principal". Search and select the name of the app created in step 2.
8. Configuration is now complete. Go back to "App Registrations" in the Active Directory view to check details like tenant ID, application ID and endpoints. You'll need those when configuring OPA (see [Example Configuration](#example-opa-configuration) below).

#### Testing Authentication

Use Curl to test client authentication with a secret.

```shell
curl --silent \
     --data "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET&scope=https://storage.azure.com/.default" \
     "https://login.microsoftonline.com/$TENANT_ID/oauth2/v2.0/token"
```

### Upload Bundle

Uploading bundles to Azure Blob storage is easily done using the [azcopy](https://docs.microsoft.com/en-us/azure/storage/common/storage-use-azcopy-v10) tool. Make sure to first properly [authorize](https://docs.microsoft.com/en-us/azure/storage/common/storage-use-azcopy-authorize-azure-active-directory) the user to be able to upload to Blob storage. 

By now you should be able to login interactively using `azcopy login --tenant-id <Active Directory tenant ID>`. Since you'll most likely will want to log in from scripts (to upload bundles programmatically), you should however create an Azure AD application, and a [service principal](https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal) to do so. Good news! If you've followed the Authentication steps above, you already have one.

**Uploading bundle using client secret authentication**
```shell
AZCOPY_SPA_CLIENT_SECRET='<application_client_secret>' azcopy login \
  --service-principal \
  --tenant-id <tenant-id> \
  --application-id <application-id>

azcopy copy bundle.tar.gz https://<storage-account-id>.blob.core.windows.net/<container-id>/bundle.tar.gz
```

**Uploading bundle using client certificate authentication**
```shell
AZCOPY_SPA_CERT_PASSWORD='<client_cert_password>' azcopy login \
  --service-principal \
  --tenant-id <tenant-id> \
  --certificate-path <path-to-certificate-file> --tenant-id <tenant-id>

azcopy copy bundle.tar.gz https://<storage-account-id>.blob.core.windows.net/<container-id>/bundle.tar.gz
```

**Uploading bundle using Curl**
```shell
token=$(curl --silent \
             --data "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET&scope=https://storage.azure.com/.default" \
             "https://login.microsoftonline.com/$TENANT_ID/oauth2/v2.0/token" | jq -r .access_token)

curl --silent \
     -X PUT \
     --data-binary "@bundle.tar.gz" -H "X-Ms-Version: 2020-04-08" -H "Authorization: Bearer $token" \
     https://styra.blob.core.windows.net/opa/bundle.tar.gz
```

### Example OPA Configuration

#### Azure Blob Storage Bundle and Client Credentials Authentication

```yaml
services:
  blob:
    url: https://my-storage-account.blob.core.windows.net
    headers:
      # This header _must_ be present in all authenticated requests
      x-ms-version: "2020-04-08"
    credentials:
      oauth2:
        token_url: "https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/token"
        client_id: "${CLIENT_ID}"
        client_secret: "${CLIENT_SECRET}"
        scopes:
          - https://storage.azure.com/.default

bundles:
  authz:
    service: blob
    resource: my-container/bundle.tar.gz
```
Note that the `$CLIENT_ID` is what is referred to as the "Application ID" inside your Azure account.

#### Azure Blob Storage Bundle and Client Credentials JWT Authentication

```yaml
keys:
  blob_key:
    algorithm: RS256
    private_key: "${PRIVATE_KEY_PEM}"

services:
  blob:
    url: https://my-storage-account.blob.core.windows.net
    headers:
      # This header _must_ be present in all authenticated requests
      x-ms-version: "2020-04-08"
    credentials:
      oauth2:
        token_url: "https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/token"
        signing_key: blob_key
        thumbprint: "8F1BDDDE9982299E62749C20EDDBAAC57F619D04"
        include_jti_claim: true
        scopes:
          - https://storage.azure.com/.default
        additional_claims:
          aud: "https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/token"
          iss: "${CLIENT_ID}"
          sub: "${CLIENT_ID}"

bundles:
  authz:
    service: blob
    resource: opa/bundle.tar.gz
```
Note that the `$CLIENT_ID` is what is referred to as the "Application ID" inside your Azure account. 
Also note in particular how the `thumbprint` property is required for Azure. The value expected here can be found under "Certificates and Secrets" in your application's configuration.

{{< figure src="thumbprint.png" width="150" caption="Certificate thumbprint" >}}

## Nginx

Nginx offers a simple but competent bundle server for those who prefer to host their own. A great choice or for local testing.

| Feature | Supported |
|---------|-----------|
| Caching headers | Yes |
| Authentication methods | [Bearer Token](https://www.openpolicyagent.org/docs/latest/configuration/#bearer-token) <sup>1</sup><br> [OAuth2 Client Credentials JWT authentication](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-client-credentials-jwt-authentication) <sup>2</sup> |

<sup>1</sup>Nginx does not support bearer token authentication, but it does support [basic auth](https://docs.nginx.com/nginx/admin-guide/security-controls/configuring-http-basic-authentication/). This can be achieved by setting `services[_].credentials.bearer.scheme` to `Basic` in the OPA configuration, and simply provide the base64 encoded credentials as the token.<br>
<sup>2</sup>Only available with Nginx Plus.

### Upload Bundle

Either use the [nginx-upload-module](https://www.nginx.com/resources/wiki/modules/upload/) or upload bundles out-of-band with SSH or similar. 

### Example OPA Configuration

```yaml
services:
  nginx:
    url: https://my-nginx.example.com
    credentials:
      bearer:
        token: dGVzdGluZzp0ZXN0aW5n
        scheme: Basic

bundles:
  authz:
    service: nginx
    resource: /bundle.tar.gz
```