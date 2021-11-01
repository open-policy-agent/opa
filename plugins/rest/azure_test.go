package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/keys"
)

func assertStringsEqual(t *testing.T, expected string, actual string, label string) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s: expected %s, got %s", label, expected, actual)
	}
}

func assertParamsEqual(t *testing.T, expected url.Values, actual url.Values, label string) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s: expected %s, got %s", label, expected.Encode(), actual.Encode())
	}
}

func TestAzureManagedIdentitiesAuthPlugin_NewClient(t *testing.T) {
	tests := []struct {
		label      string
		endpoint   string
		apiVersion string
		resource   string
		objectID   string
		clientID   string
		miResID    string
	}{
		{
			"test all defaults",
			"", "", "", "", "", "",
		},
		{
			"test no defaults",
			"some_endpoint", "some_version", "some_resource", "some_oid", "some_cid", "some_miresid",
		},
	}

	nonEmptyString := func(value string, defaultValue string) string {
		if value == "" {
			return defaultValue
		}
		return value
	}

	for _, tt := range tests {
		config := generateConfigString(tt.endpoint, tt.apiVersion, tt.resource, tt.objectID, tt.clientID, tt.miResID)

		client, err := New([]byte(config), map[string]*keys.Config{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		ap := client.config.Credentials.AzureManagedIdentity
		_, err = ap.NewClient(client.config)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// We test that default values are set correctly in the azureManagedIdentitiesAuthPlugin
		// Note that there is significant overlap between TestAzureManagedIdentitiesAuthPlugin_NewClient and TestAzureManagedIdentitiesAuthPlugin
		// This is because the latter cannot test default endpoint setting, which we do here
		assertStringsEqual(t, nonEmptyString(tt.endpoint, azureIMDSEndpoint), ap.Endpoint, tt.label)
		assertStringsEqual(t, nonEmptyString(tt.apiVersion, defaultAPIVersion), ap.APIVersion, tt.label)
		assertStringsEqual(t, nonEmptyString(tt.resource, defaultResource), ap.Resource, tt.label)
		assertStringsEqual(t, tt.objectID, ap.ObjectID, tt.label)
		assertStringsEqual(t, tt.clientID, ap.ClientID, tt.label)
		assertStringsEqual(t, tt.miResID, ap.MiResID, tt.label)
	}
}

func TestAzureManagedIdentitiesAuthPlugin(t *testing.T) {
	tests := []struct {
		label          string
		apiVersion     string
		resource       string
		objectID       string
		clientID       string
		miResID        string
		expectedParams url.Values
	}{
		{
			"test all defaults",
			"", "", "", "", "",
			url.Values{
				"api-version": []string{"2018-02-01"},
				"resource":    []string{"https://storage.azure.com/"},
			},
		},
		{
			"test custom api version",
			"2021-02-01", "", "", "", "",
			url.Values{
				"api-version": []string{"2021-02-01"},
				"resource":    []string{"https://storage.azure.com/"},
			},
		},
		{
			"test custom resource",
			"", "https://management.azure.com/", "", "", "",
			url.Values{
				"api-version": []string{"2018-02-01"},
				"resource":    []string{"https://management.azure.com/"},
			},
		},
		{
			"test custom IDs",
			"", "", "oid", "cid", "mrid",
			url.Values{
				"api-version": []string{"2018-02-01"},
				"resource":    []string{"https://storage.azure.com/"},
				"object_id":   []string{"oid"},
				"client_id":   []string{"cid"},
				"mi_res_id":   []string{"mrid"},
			},
		},
	}

	for _, tt := range tests {
		ts := azureManagedIdentitiesTestServer{
			t:              t,
			label:          tt.label,
			expectedParams: tt.expectedParams,
		}
		ts.start()

		config := generateConfigString(ts.server.URL, tt.apiVersion, tt.resource, tt.objectID, tt.clientID, tt.miResID)

		client, err := New([]byte(config), map[string]*keys.Config{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		ctx := context.Background()
		_, _ = client.Do(ctx, "GET", "test")
		ts.stop()
	}
}

type azureManagedIdentitiesTestServer struct {
	t              *testing.T
	server         *httptest.Server
	label          string
	expectedParams url.Values
}

func (t *azureManagedIdentitiesTestServer) handle(_ http.ResponseWriter, r *http.Request) {
	assertParamsEqual(t.t, t.expectedParams, r.URL.Query(), t.label)
}

func (t *azureManagedIdentitiesTestServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *azureManagedIdentitiesTestServer) stop() {
	t.server.Close()
}

func generateConfigString(endpoint, apiVersion, resource, objectID, clientID, miResID string) string {
	return fmt.Sprintf(`{
			"name": "name",
			"url": "url",
			"allow_insecure_tls": true,
			"credentials": {
				"azure_managed_identity": {
					"endpoint": "%s",
					"api_version": "%s",
					"resource": "%s",
					"object_id": "%s",
					"client_id": "%s",
					"mi_res_id": "%s"
				}
			}
		}`, endpoint, apiVersion, resource, objectID, clientID, miResID)
}
