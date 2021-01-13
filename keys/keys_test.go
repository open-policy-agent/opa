package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestParseKeysConfig(t *testing.T) {

	key := `-----BEGIN PUBLIC KEY----- MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA7nJwME0QNM6g0Ou9Sylj lcIY4cnBcs8oWVHe74bJ7JTgYmDOk2CA14RE3wJNkUKERP/cRdesKDA/BToJXJUr oYvhjXxUYn+i3wK5vOGRY9WUtTF9paIIpIV4USUOwDh3ufhA9K3tyh+ZVsqn80em 0Lj2ME0EgScuk6u0/UYjjNvcmnQl+uDmghG8xBZh7TZW2+aceMwlb4LJIP36VRhg jKQGIxg2rW8ROXgJaFbNRCbiOUUqlq9SUZuhHo8TNOARXXxp9R4Fq7Cl7ZbwWtNP wAtM1y+Z+iyu/i91m0YLlU2XBOGLu9IA8IZjPlbCnk/SygpV9NNwTY9DSQ0QfXcP TGlsbFwzRzTlhH25wEl3j+2Ub9w/NX7Yo+j/Ei9eGZ8cq0bcvEwDeIo98HeNZWrL UUArayRYvh8zutOlzqehw8waFk9AxpfEp9oWekSz8gZw9OL773EhnglYxxjkPHNz k66CufLuTEf6uE9NLE5HnlQMbiqBFirIyAWGKyU3v2tphKvcogxmzzWA51p0GY0l GZvlLNt2NrJv2oGecyl3BLqHnBi+rGAosa/8XgfQT8RIk7YR/tDPDmPfaqSIc0po +NcHYEH82Yv+gfKSK++1fyssGCsSRJs8PFMuPGgv62fFrE/EHSsHJaNWojSYce/T rxm2RaHhw/8O4oKcfrbaRf8CAwEAAQ== -----END PUBLIC KEY-----`

	config := fmt.Sprintf(`{"foo": {"algorithm": "HS256", "key": "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ"},
			  				"bar": {"key": %v}
				}`, key)

	tests := map[string]struct {
		input   string
		result  map[string]*Config
		wantErr bool
		err     error
	}{
		"valid_config_one_key": {
			`{"foo": {"algorithm": "HS256", "key": "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ"}}`,
			map[string]*Config{"foo": {Key: "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ", Algorithm: "HS256"}},
			false, nil,
		},
		"valid_config_two_key": {
			config,
			map[string]*Config{
				"foo": {Key: "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ", Algorithm: "HS256"},
				"bar": {Key: key, Algorithm: "RS256"},
			},
			false, nil,
		},
		"invalid_config_no_key": {
			`{"foo": {"algorithm": "HS256"}}`,
			nil,
			true, fmt.Errorf("invalid keys configuration: no keys provided for key ID foo"),
		},
		"valid_config_default_alg": {
			`{"foo": {"key": "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ"}}`,
			map[string]*Config{"foo": {Key: "FdFYFzERwC2uCBB46pZQi4GG85LujR8obt-KWRBICVQ", Algorithm: "RS256"}},
			false, nil,
		},
		"invalid_raw_key_config": {
			`{"bar": [1,2,3]}`,
			nil,
			true, fmt.Errorf("json: cannot unmarshal array into Go value of type keys.Config"),
		},
		"invalid_raw_config": {
			`[1,2,3]`,
			nil,
			true, fmt.Errorf("json: cannot unmarshal array into Go value of type map[string]json.RawMessage"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			kc, err := ParseKeysConfig([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if !reflect.DeepEqual(kc, tc.result) {
				t.Fatalf("Expected key config %v but got %v", tc.result, kc)
			}
		})
	}
}

func TestNewKeyConfig(t *testing.T) {
	publicKey := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9KaakMv1XKKDaSch3PFR
3a27oaHp1GNTTNqvb1ZaHZXp+wuhYDwc/MTE67x9GCifvQBWzEGorgTq7aisiOyl
vKifwz6/wQ+62WHKG/sqKn2Xikp3P63aBIPlZcHbkyyRmL62yeyuzYoGvLEYel+m
z5SiKGBwviSY0Th2L4e5sGJuk2HOut6emxDi+E2Fuuj5zokFJvIT6Urlq8f3h6+l
GeR6HUOXqoYVf7ff126GP7dticTVBgibxkkuJFmpvQSW6xmxruT4k6iwjzbZHY7P
ypZ/TdlnuGC1cOpAVyU7k32IJ9CRbt3nwEf5U54LRXLLQjFixWZHwKdDiMTF4ws0
+wIDAQAB
-----END PUBLIC KEY-----`

	files := map[string]string{
		"public.pem": publicKey,
	}

	test.WithTempFS(files, func(rootDir string) {

		kc, err := NewKeyConfig(filepath.Join(rootDir, "public.pem"), "RS256", "read")
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		expected := &Config{
			Key:       publicKey,
			Algorithm: "RS256",
			Scope:     "read",
		}

		if !reflect.DeepEqual(kc, expected) {
			t.Fatalf("Expected key config %v but got %v", expected, kc)
		}

		// secret provided on command-line
		kc, err = NewKeyConfig(publicKey, "HS256", "")
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		expected = &Config{
			Key:       publicKey,
			Algorithm: "HS256",
			Scope:     "",
		}

		if !reflect.DeepEqual(kc, expected) {
			t.Fatalf("Expected key config %v but got %v", expected, kc)
		}

		// simulate error while reading file
		err = os.Chmod(filepath.Join(rootDir, "public.pem"), 0111)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		_, err = NewKeyConfig(filepath.Join(rootDir, "public.pem"), "RS256", "read")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})
}

func TestKeyConfigEqual(t *testing.T) {
	tests := map[string]struct {
		a   *Config
		b   *Config
		exp bool
	}{
		"equal": {
			&Config{
				Key:       "foo",
				Algorithm: "RS256",
				Scope:     "read",
			},
			&Config{
				Key:       "foo",
				Algorithm: "RS256",
				Scope:     "read",
			},
			true,
		},
		"not_equal": {
			&Config{
				Key:       "foo",
				Algorithm: "RS256",
				Scope:     "read",
			},
			&Config{
				Key:       "foo",
				Algorithm: "RS256",
				Scope:     "write",
			},
			false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := tc.a.Equal(tc.b)

			if actual != tc.exp {
				t.Fatalf("Expected config equal result %v but got %v", tc.exp, actual)
			}
		})
	}
}
