package test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockBundle sets a bundle named file on the test server containing the given
// policies.
func MockBundle(file string, policies map[string]string) func(*Server) error {
	return func(s *Server) error {
		if !strings.HasPrefix(file, "/bundles/") {
			return fmt.Errorf("mock bundle filename must be prefixed with '/bundle/ but got %q", file)
		}
		s.bundles[file] = policies
		return nil
	}
}

// MockOCIBundle prepares the server to allow serving "/v2" OCI responses from the supplied policies
// Ref parameter must be in the form of <registry>/<org>/<repo>:<tag> that will be used in detecting future calls
func MockOCIBundle(ref string, policies map[string]string) func(*Server) error {
	return func(s *Server) error {
		if !strings.Contains(ref, "/") {
			return fmt.Errorf("mock oci bundle ref must contain 'org/repo' but got %q", ref)
		}
		return s.buildBundles(ref, policies)
	}
}

// Ready provides a channel that the server will use to gate readiness. The
// caller can provide this channel to prevent the server from becoming ready.
// The server will response with HTTP 500 responses until ready. The caller
// should close the channel to indicate readiness.
func Ready(ch chan struct{}) func(*Server) error {
	return func(s *Server) error {
		s.ready = ch
		return nil
	}
}

// Server provides a mock HTTP server for testing the SDK and integrations.
type Server struct {
	server  *httptest.Server
	ready   chan struct{}
	bundles map[string]map[string]string
}

// MustNewServer returns a new Server for test purposes or panics if an error occurs.
func MustNewServer(opts ...func(*Server) error) *Server {
	s, err := NewServer(opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// NewServer returns a new Server for test purposes.
func NewServer(opts ...func(*Server) error) (*Server, error) {
	s := &Server{
		bundles: map[string]map[string]string{},
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	if s.ready == nil {
		s.ready = make(chan struct{})
		close(s.ready)
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s, nil
}

// WithTestBundle adds a bundle to the server at the specified endpoint.
func (s *Server) WithTestBundle(endpoint string, policies map[string]string) *Server {
	s.bundles[endpoint] = policies
	return s
}

// Stop stops the test server.
func (s *Server) Stop() {
	s.server.Close()
}

// URL returns the base URL of the server.
func (s *Server) URL() string {
	return s.server.URL
}

// Builds the tarball from the supplied policies and prepares the layers in a temporary directory
func (s *Server) buildBundles(ref string, policies map[string]string) error {
	// Prepare the modules to include in the bundle. Sort them so bundles are deterministic.
	modules := make([]bundle.ModuleFile, 0, len(policies))
	for url, str := range policies {
		module, err := ast.ParseModule(url, str)
		if err != nil {
			return fmt.Errorf("failed to parse module: %v", err)
		}
		modules = append(modules, bundle.ModuleFile{
			URL:    url,
			Parsed: module,
		})
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].URL < modules[j].URL
	})

	// Compile the bundle out into a buffer
	buf := bytes.NewBuffer(nil)
	err := compile.New().WithOutput(buf).WithBundle(&bundle.Bundle{
		Data:    map[string]interface{}{},
		Modules: modules,
	}).Build(context.Background())
	if err != nil {
		return err
	}
	directoryName, err := os.MkdirTemp("", "oci-test-temp")
	fmt.Println("Testing OCI temporary directory:", directoryName)
	if err != nil {
		return err
	}
	// Write buf tarball to layer
	tarLayer := filepath.Join(directoryName, "tar.layer")
	err = os.WriteFile(tarLayer, buf.Bytes(), 0655)
	if err != nil {
		return err
	}
	// Write empty config layer
	configLayer := filepath.Join(directoryName, "config.layer")
	err = os.WriteFile(configLayer, []byte("{}"), 0655)
	if err != nil {
		return err
	}
	// Calculate SHA and size and prepare manifest layer
	tarSHA, err := getFileSHA(tarLayer)
	if err != nil {
		return err
	}

	configSHA, err := getFileSHA(configLayer)
	if err != nil {
		return err
	}

	var manifest ocispec.Manifest
	manifest.SchemaVersion = 2
	manifest.Config = ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    digest.Digest(fmt.Sprintf("sha256:%x", configSHA)),
		Size:      int64(2), // config size is set to 2 as an empty config is used
	}
	manifest.Layers = []ocispec.Descriptor{
		{
			MediaType: ocispec.MediaTypeImageLayerGzip,
			Digest:    digest.Digest(fmt.Sprintf("sha256:%x", tarSHA)),
			Size:      int64(buf.Len()),
			Annotations: map[string]string{
				ocispec.AnnotationTitle:   ref,
				ocispec.AnnotationCreated: time.Now().Format(time.RFC3339),
			},
		},
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	manifestLayer := filepath.Join(directoryName, "manifest.layer")
	err = os.WriteFile(manifestLayer, manifestData, 0655)
	if err != nil {
		return err
	}

	// Set ref layer paths to server bundles
	s.bundles[ref] = map[string]string{
		"manifest": manifestLayer,
		"config":   configLayer,
		"tar":      tarLayer,
	}
	return nil
}

func getFileSHA(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {

	select {
	case <-s.ready:
	default:
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/v2") {
		s.handleOCIBundles(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/bundles") {
		s.handleBundles(w, r)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}

func (s *Server) handleOCIBundles(w http.ResponseWriter, r *http.Request) {
	ref := ""  // key used to detect layers from s.bundles
	tag := ""  // image tag used in request path verification
	repo := "" // image repo used in request path verification
	var buf bytes.Buffer
	//get first key that matches request url pattern
	for key := range s.bundles {
		// extract tag
		parsedRef := strings.Split(key, ":")
		checkRef := strings.Split(parsedRef[0], "/")
		// check if request path contains org and repository
		if strings.Contains(r.URL.Path, checkRef[1]) && strings.Contains(r.URL.Path, checkRef[2]) {
			ref = key
			tag = parsedRef[1]
			repo = checkRef[1] + "/" + checkRef[2]
			break
		}
	}
	if ref == "" || tag == "" || repo == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	layers := s.bundles[ref]
	fi, err := os.Stat(layers["manifest"])
	if err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	manifestSize := fi.Size()
	manifestSHA, err := getFileSHA(layers["manifest"])
	if err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	fi, err = os.Stat(layers["config"])
	if err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	configSize := fi.Size()
	configSHA, err := getFileSHA(layers["config"])
	if err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	fi, err = os.Stat(layers["tar"])
	if err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	// get the size
	tarSize := fi.Size()
	tarSHA, err := getFileSHA(layers["tar"])
	if err != nil {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}

	if r.URL.Path == fmt.Sprintf("/v2/%s/manifests/%s", repo, tag) {
		w.Header().Add("Content-Length", fmt.Sprintf("%d", manifestSize))
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", fmt.Sprintf("sha256:%x", manifestSHA))
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.URL.Path == fmt.Sprintf("/v2/%s/manifests/sha256:%x", repo, manifestSHA) {
		w.Header().Add("Content-Length", fmt.Sprintf("%d", manifestSize))
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", fmt.Sprintf("sha256:%x", manifestSHA))
		w.WriteHeader(200)
		bs, err := os.ReadFile(layers["manifest"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		buf.WriteString(string(bs))
		_, err = w.Write(buf.Bytes())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if r.URL.Path == fmt.Sprintf("/v2/%s/blobs/sha256:%x", repo, configSHA) {
		w.Header().Add("Content-Length", fmt.Sprintf("%d", configSize))
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", fmt.Sprintf("sha256:%x", configSHA))
		w.WriteHeader(200)
		bs, err := os.ReadFile(layers["config"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		buf.WriteString(string(bs))
		_, err = w.Write(buf.Bytes())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if r.URL.Path == fmt.Sprintf("/v2/%s/blobs/sha256:%x", repo, tarSHA) {
		w.Header().Add("Content-Length", fmt.Sprintf("%d", tarSize))
		w.Header().Add("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Add("Docker-Content-Digest", fmt.Sprintf("sha256:%x", tarSHA))
		w.WriteHeader(200)
		bs, err := os.ReadFile(layers["tar"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		buf.WriteString(string(bs))
		_, err = w.Write(buf.Bytes())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
}

func (s *Server) handleBundles(w http.ResponseWriter, r *http.Request) {

	// Return 404 if bundle path does not exist.
	b, ok := s.bundles[r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Prepare a mapping to store bundle data
	data := map[string]interface{}{}

	// Prepare a manifest for use if a .manifest file exists.
	var manifest bundle.Manifest

	// Prepare the modules to include in the bundle. Sort them so bundles are deterministic.
	modules := make([]bundle.ModuleFile, 0, len(b))
	for url, str := range b {
		switch {
		case url == ".manifest":
			err := json.Unmarshal([]byte(str), &manifest)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "error unmarshaling .manifest file: %v", err)
				return
			}
		case strings.HasSuffix(url, ".rego"):
			module, err := ast.ParseModule(url, str)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			modules = append(modules, bundle.ModuleFile{
				URL:    url,
				Parsed: module,
			})
		case strings.HasSuffix(url, ".json"):
			if strings.Contains(url, "/") {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "nested data documents are not implemented in the dummy server: %s", url)
				return
			}

			var d map[string]interface{}

			err := json.Unmarshal([]byte(str), &d)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "error unmarshaling json file: %v", err)
				return
			}

			for k, v := range d {
				data[k] = v
			}
		default:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "unexpected file in dummy bundle: %s", url)
			return
		}
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].URL < modules[j].URL
	})

	// Compile the bundle out into a buffer
	buf := bytes.NewBuffer(nil)
	err := compile.New().WithOutput(buf).WithBundle(&bundle.Bundle{
		Data:     data,
		Modules:  modules,
		Manifest: manifest,
	}).Build(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	// Write out the bundle
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, buf)
}
