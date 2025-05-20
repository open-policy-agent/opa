package rest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	azureIMDSEndpoint                 = "http://169.254.169.254/metadata/identity/oauth2/token"
	defaultAPIVersion                 = "2018-02-01"
	defaultResource                   = "https://storage.azure.com/"
	timeout                           = 5 * time.Second
	defaultAPIVersionForAppServiceMsi = "2019-08-01"
	defaultKeyVaultAPIVersion         = "7.4"
)

// azureManagedIdentitiesToken holds a token for managed identities for Azure resources
type azureManagedIdentitiesToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
	ExpiresOn   string `json:"expires_on"`
	NotBefore   string `json:"not_before"`
	Resource    string `json:"resource"`
	TokenType   string `json:"token_type"`
}

// azureManagedIdentitiesError represents an error fetching an azureManagedIdentitiesToken
type azureManagedIdentitiesError struct {
	Err         string `json:"error"`
	Description string `json:"error_description"`
	Endpoint    string
	StatusCode  int
}

func (e *azureManagedIdentitiesError) Error() string {
	return fmt.Sprintf("%v %s retrieving azure token from %s: %s", e.StatusCode, e.Err, e.Endpoint, e.Description)
}

// azureManagedIdentitiesAuthPlugin uses an azureManagedIdentitiesToken.AccessToken for bearer authorization
type azureManagedIdentitiesAuthPlugin struct {
	Endpoint         string `json:"endpoint"`
	APIVersion       string `json:"api_version"`
	Resource         string `json:"resource"`
	ObjectID         string `json:"object_id"`
	ClientID         string `json:"client_id"`
	MiResID          string `json:"mi_res_id"`
	UseAppServiceMsi bool   `json:"use_app_service_msi,omitempty"`
}

func (ap *azureManagedIdentitiesAuthPlugin) setDefaults() {
	if ap.Endpoint == "" {
		identityEndpoint := os.Getenv("IDENTITY_ENDPOINT")
		if identityEndpoint != "" {
			ap.UseAppServiceMsi = true
			ap.Endpoint = identityEndpoint
		} else {
			ap.Endpoint = azureIMDSEndpoint
		}
	}

	if ap.Resource == "" {
		ap.Resource = defaultResource
	}

	if ap.APIVersion == "" {
		if ap.UseAppServiceMsi {
			ap.APIVersion = defaultAPIVersionForAppServiceMsi
		} else {
			ap.APIVersion = defaultAPIVersion
		}
	}

}

func (ap *azureManagedIdentitiesAuthPlugin) NewClient(c Config) (*http.Client, error) {
	if c.Type == "oci" {
		return nil, errors.New("azure managed identities auth: OCI service not supported")
	}
	ap.setDefaults()
	t, err := DefaultTLSConfig(c)
	if err != nil {
		return nil, err
	}

	return DefaultRoundTripperClient(t, *c.ResponseHeaderTimeoutSeconds), nil
}

func (ap *azureManagedIdentitiesAuthPlugin) Prepare(req *http.Request) error {
	token, err := azureManagedIdentitiesTokenRequest(
		ap.Endpoint, ap.APIVersion, ap.Resource,
		ap.ObjectID, ap.ClientID, ap.MiResID,
		ap.UseAppServiceMsi,
	)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	return nil
}

// azureManagedIdentitiesTokenRequest fetches an azureManagedIdentitiesToken
func azureManagedIdentitiesTokenRequest(
	endpoint, apiVersion, resource, objectID, clientID, miResID string,
	useAppServiceMsi bool,
) (azureManagedIdentitiesToken, error) {
	var token azureManagedIdentitiesToken
	e := buildAzureManagedIdentitiesRequestPath(endpoint, apiVersion, resource, objectID, clientID, miResID)

	request, err := http.NewRequest("GET", e, nil)
	if err != nil {
		return token, err
	}
	if useAppServiceMsi {
		identityHeader := os.Getenv("IDENTITY_HEADER")
		if identityHeader == "" {
			return token, errors.New("azure managed identities auth: IDENTITY_HEADER env var not found")
		}
		request.Header.Add("x-identity-header", identityHeader)
	} else {
		request.Header.Add("Metadata", "true")
	}

	httpClient := http.Client{Timeout: timeout}
	response, err := httpClient.Do(request)
	if err != nil {
		return token, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return token, err
	}

	if s := response.StatusCode; s != http.StatusOK {
		var azureError azureManagedIdentitiesError
		err = json.Unmarshal(data, &azureError)
		if err != nil {
			return token, err
		}

		azureError.Endpoint = e
		azureError.StatusCode = s
		return token, &azureError
	}

	err = json.Unmarshal(data, &token)
	if err != nil {
		return token, err
	}
	return token, nil
}

// buildAzureManagedIdentitiesRequestPath constructs the request URL for an Azure managed identities token request
func buildAzureManagedIdentitiesRequestPath(
	endpoint, apiVersion, resource, objectID, clientID, miResID string,
) string {
	params := url.Values{
		"api-version": []string{apiVersion},
		"resource":    []string{resource},
	}

	if objectID != "" {
		params.Add("object_id", objectID)
	}

	if clientID != "" {
		params.Add("client_id", clientID)
	}

	if miResID != "" {
		params.Add("mi_res_id", miResID)
	}

	return endpoint + "?" + params.Encode()
}

type azureKeyVaultSignPlugin struct {
	config  azureKeyVaultConfig
	tokener func() (string, error)
}

func newKeyVaultSignPlugin(ap *azureManagedIdentitiesAuthPlugin, cfg *azureKeyVaultConfig) *azureKeyVaultSignPlugin {
	resp := &azureKeyVaultSignPlugin{
		tokener: func() (string, error) {
			resp, err := azureManagedIdentitiesTokenRequest(
				ap.Endpoint,
				ap.APIVersion,
				cfg.URL.String(),
				ap.ObjectID,
				ap.ClientID,
				ap.MiResID,
				ap.UseAppServiceMsi)
			if err != nil {
				return "", err
			}
			return resp.AccessToken, nil
		},
		config: *cfg,
	}
	return resp
}

func (akv *azureKeyVaultSignPlugin) setDefaults() {
	if akv.config.APIVersion == "" {
		akv.config.APIVersion = defaultKeyVaultAPIVersion
	}
}

type kvRequest struct {
	Alg   string `json:"alg"`
	Value string `json:"value"`
}

type kvResponse struct {
	KID   string `json:"kid"`
	Value string `json:"value"`
}

// SignDigest() uses the Microsoft keyvault rest api to sign a byte digest
// https://learn.microsoft.com/en-us/rest/api/keyvault/keys/sign/sign
func (ap *azureKeyVaultSignPlugin) SignDigest(ctx context.Context, digest []byte) (string, error) {
	tkn, err := ap.tokener()
	if err != nil {
		return "", err
	}
	if ap.config.URL.Host == "" {
		return "", errors.New("keyvault host not set")
	}

	signingURL := ap.config.URL.JoinPath("keys", ap.config.Key, ap.config.KeyVersion, "sign")
	q := signingURL.Query()
	q.Set("api-version", ap.config.APIVersion)
	signingURL.RawQuery = q.Encode()
	reqBody, err := json.Marshal(kvRequest{
		Alg:   ap.config.Alg,
		Value: base64.StdEncoding.EncodeToString(digest)})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, signingURL.String(), bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+tkn)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.Body != nil {
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("non 200 status code, got: %d. Body: %v", resp.StatusCode, string(b))
		}
		return "", fmt.Errorf("non 200 status code from keyvault sign, got: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("failed to read keyvault response body")
	}

	var res kvResponse
	err = json.Unmarshal(respBytes, &res)
	if err != nil {
		return "", fmt.Errorf("no valid keyvault response, got: %v", string(respBytes))
	}

	return res.Value, nil
}
