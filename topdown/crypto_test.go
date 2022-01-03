package topdown

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwk"
)

func TestX509ParseAndVerify(t *testing.T) {
	rootCA := `-----BEGIN CERTIFICATE-----
MIIBoDCCAUagAwIBAgIRAJXcMYZALXooNq/VV/grXhMwCgYIKoZIzj0EAwIwLjER
MA8GA1UEChMIT1BBIFRlc3QxGTAXBgNVBAMTEE9QQSBUZXN0IFJvb3QgQ0EwHhcN
MjEwNzAxMTc0MTUzWhcNMzEwNjI5MTc0MTUzWjAuMREwDwYDVQQKEwhPUEEgVGVz
dDEZMBcGA1UEAxMQT1BBIFRlc3QgUm9vdCBDQTBZMBMGByqGSM49AgEGCCqGSM49
AwEHA0IABFqhdZA5LjsJgzsBvhgzfayZFOk+C7PmGCi7xz6zOC3xWORJZSNOyZeJ
YzSKFmoMZkcFMfslTW1jp9fwe1xl3HWjRTBDMA4GA1UdDwEB/wQEAwIBBjASBgNV
HRMBAf8ECDAGAQH/AgEBMB0GA1UdDgQWBBTch60qxQvLl+AfDfcaXmjvT8GvpzAK
BggqhkjOPQQDAgNIADBFAiBqraIP0l2U0oNuH0+rf36hDks94wSB5EGlGH3lYNMR
ugIhANkbukX5hOP8pJDRWP/pYuv6MBnRY4BS8gpp9Vu31qOb
-----END CERTIFICATE-----`
	intermediateCA := `-----BEGIN CERTIFICATE-----
MIIByDCCAW6gAwIBAgIQC0k4DPGrh9me73EJX5zntTAKBggqhkjOPQQDAjAuMREw
DwYDVQQKEwhPUEEgVGVzdDEZMBcGA1UEAxMQT1BBIFRlc3QgUm9vdCBDQTAeFw0y
MTA3MDExNzQxNTNaFw0zMTA2MjkxNzQxNTNaMDYxETAPBgNVBAoTCE9QQSBUZXN0
MSEwHwYDVQQDExhPUEEgVGVzdCBJbnRlcm1lZGlhdGUgQ0EwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAARvXQa7fy476gDI81nqLYb2SnD459WxBmU0hk2bA3ZuNtI+
H20KXz6ISmxH3MZ2WBm6rOy7y4Gn+WMCJuxzcl5jo2YwZDAOBgNVHQ8BAf8EBAMC
AQYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUuslZNjJl0V8I1Gj17IID
ALy/9WEwHwYDVR0jBBgwFoAU3IetKsULy5fgHw33Gl5o70/Br6cwCgYIKoZIzj0E
AwIDSAAwRQIgUwsYApW9Tsm6AstWswaKGie0srB4FUkUbfKwWmUI2JgCIQCBTySN
MF+EiQAMKyz/N9KUuXEckC356WvKcyJaYYcV0w==
-----END CERTIFICATE-----`
	leaf := `-----BEGIN CERTIFICATE-----
MIIB8zCCAZqgAwIBAgIRAID4gPKg7DDiuOfzUYFSXLAwCgYIKoZIzj0EAwIwNjER
MA8GA1UEChMIT1BBIFRlc3QxITAfBgNVBAMTGE9QQSBUZXN0IEludGVybWVkaWF0
ZSBDQTAeFw0yMTA3MDUxNzQ5NTBaFw0zNjA3MDExNzQ5NDdaMCUxIzAhBgNVBAMT
Gm5vdGFyZWFsc2l0ZS5vcGEubG9jYWxob3N0MFkwEwYHKoZIzj0CAQYIKoZIzj0D
AQcDQgAE1YSXZXeaGGL+XeYyoPi/QdA39Ds4fgxSHJTMh+js393kByPm2PNtFkem
tUii3KCRJw3SEh3z0JWr/9y4+ua2L6OBmTCBljAOBgNVHQ8BAf8EBAMCB4AwHQYD
VR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBRL0P0g17viZHo9
CnXe3ZQJm48LXTAfBgNVHSMEGDAWgBS6yVk2MmXRXwjUaPXsggMAvL/1YTAlBgNV
HREEHjAcghpub3RhcmVhbHNpdGUub3BhLmxvY2FsaG9zdDAKBggqhkjOPQQDAgNH
ADBEAiAtmZewL94ijN0YwUGaJM9BXCaoTQPwkzugqjCj+K912QIgKKFvbPu4asrE
nwy7dzejHmQUcZ/aUNbc4VTbiv15ESk=
-----END CERTIFICATE-----`

	t.Run("TestFullChainPEM", func(t *testing.T) {
		chain := strings.Join([]string{rootCA, intermediateCA, leaf}, "\n")

		parsed, err := getX509CertsFromString(chain)
		if err != nil {
			t.Fatalf("failed to parse PEM cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err != nil {
			t.Error("x509 verification failed when it was expected to succeed")
		}
	})

	t.Run("TestFullChainBase64", func(t *testing.T) {
		chain := strings.Join([]string{rootCA, intermediateCA, leaf}, "\n")
		b64 := base64.StdEncoding.EncodeToString([]byte(chain))

		parsed, err := getX509CertsFromString(b64)
		if err != nil {
			t.Fatalf("failed to parse base64 cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err != nil {
			t.Error("x509 verification failed when it was expected to succeed")
		}
	})

	t.Run("TestWrongOrder", func(t *testing.T) {
		chain := strings.Join([]string{leaf, intermediateCA, rootCA}, "\n")

		parsed, err := getX509CertsFromString(chain)
		if err != nil {
			t.Fatalf("failed to parse PEM cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err == nil {
			t.Error("x509 verification succeeded when it was expected to fail")
		}
	})

	t.Run("TestMissingIntermediate", func(t *testing.T) {
		chain := strings.Join([]string{rootCA, leaf}, "\n")

		parsed, err := getX509CertsFromString(chain)
		if err != nil {
			t.Fatalf("failed to parse PEM cert chain: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err == nil {
			t.Error("x509 verification succeeded when it was expected to fail")
		}
	})

	t.Run("TestTooFewCerts", func(t *testing.T) {
		parsed, err := getX509CertsFromString(leaf)
		if err != nil {
			t.Fatalf("failed to parse leaf cert: %v", err)
		}

		if _, err := verifyX509CertificateChain(parsed); err == nil {
			t.Error("x509 verification succeeded when it was expected to fail")
		}
	})
}

func TestParseRSAPrivateKey(t *testing.T) {
	rsaPrivateKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA3Y8cXdK06ufUSP035jiwJk8IsuwGjJD/LSRvE2AhJL/Vp9mu
41z1bV5Mi/TTK/uZNqv6VdvTxFPZOUYycLXEchg8L6wrOLgAX0DleP+YTKGG4oyg
dTZZcqzwr4p7WhYzLFmpW8RCLgHJbV0fF1pejJKtV+9fpsdX8oQzKvqO39ne1hl+
m/lq2LKBK0z03c4ay+bFzA8AFMndmzfB3uXl2fTFsNaoYxAkGwlcvFAXNegPKtaf
9Co5JpRlRejPYVSonCvCvBakGIDCRb0ZHQrcGBzDnqjZeZMDkfe0YKoRUR+JFn69
C7a4tHheA0TerIDcv+IqadY7p2jwIom9di1oWwIDAQABAoIBAQDXEXGGvd+y20Gd
bHhTuZl8RmH6VNTypFmf92r/UuQ5aSI8Ijn7KKRw+wWxIgHPAxcyE/UYXSCOxpnp
V/Pkpv0/h7j8ydLW5v4teLCIKQws7ushhULJJO3lPG0S6Yld5IjeN1cH5lYblM5z
o95na+i16jfsUUf3fDAqERweT0Rbk7IlegTgXtXLjbvGpFWgjH7Oc8UPpy56i05h
NtdBvQhFV8LMckQAfEinBTPDHqZw6hGIfJtieRhwTzGh5H0fnDCRZanRKm2uxh4Z
9ciYZ/wa0Af23atGoax1YbQJFJK8h0vWcL1jJkaZ+CmVmRtYcWPTpDNGe2FQn9I2
EwF5nB8BAoGBAPpAsZiFC00YJf1gN4G588+7hxMU2BaoTosImSD27sLLmE2XHBa+
FrtLJR+t6pRtt7aQccGrNp2G234ucjitM2A1JmtzywPhtAXp+/VaguikdJ62zAjl
Sn6nl9W6ovOQ0NsHGmO7MFILrWXXpF7IqhXd/MdwMnxJABsKqZpBLB2BAoGBAOKl
uARPETauBRdQisEzHI1kosHigCVCSTwwTnFa8LXfinfFCq68SuuwqUdN5RaNUpGx
zTFxOgihcSlfOF0/VXROi6PI768pp2SOgbKjXsleZqxaSe5iZ61jt0uU0HlUsfoI
JXULgVweidZhlD0JJK2RGK2K7CVGTPluX07xO6vbAoGAPOPE0oF8sHNxuubQWqYu
JptQUFpAAbNN+RJMf/LVQVxcYHSmBvqVeVjdXYnpi9fuXWNj6mWIUmffvCH89MFf
wMbt5DM2cGlYbh/yiE5Pj9+D6KI9nuR7bbnFfeF9iJnx13kw+JcxOKVSuXbwrYdR
qyRqPvSTtB3nAq1jev7khwECgYBEgldHZicL4jpDu+LVV3/P9ZWFCdQ2bvz4Jpnv
hc+xCisu3O7Htr7m03W3ygHveTR2OcqOoW0rYrF0EgZVmWlZSMzI61oYFn001ia6
OsvSDqj2fCxQ1IoGTVgAjrEdm85Yh9HauWmW0NxVYxWOBY+Cr5NIEfAjrEZkN0qz
8BNbdQKBgD4w2xm7jFMUgPzHp7L8RWMWLUTBudc981dOPQJ5kAR5n2oEhE1YJs+e
GjJuyhAhz5VdHn2H2+RptQ70RVM+ctDNKYZko2aH4uGZq/6X5MWGr1erLMgMbg5q
+oSLpOUiUobapGdl9fgHetyFw/N9TI1tl/4+2uFqW5knBQnXByPP
-----END RSA PRIVATE KEY-----`

	rsaPrivateKeyPKCS8 := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDP7abKDTHtqGkk
6c/jxbZph17QcVz3NxcRrQ8RCLWHZd020oANIssGgZwGuy9hvQUfEYRy1+78wJmV
c7naeJ8qkLj1u0OsDLwofRaYXzkZUFitZr2Ygkzhy8/GVhdIMVnAV2u4LHvpw+dS
8hsnpWnIzF5Rdo3e7KNZbjZlCLBDrmorGsdvYKqwN/7aBd81YaS5dz67oacG0/bI
Bn2ox93OI+OQLrdtYG2aDMv9eEs8QQ8X10YI2Fsp2t2rAstwBGhsSbMPdBF82G9G
XIng4ZTO6P0G1ypYcXha4okhLO2ck15bYyd+EAY3QfyJ5MMcHMvr/iJpGCVeIyFm
m9qoyGqLAgMBAAECggEAdtocLYBvWq6aM1xm1YaNJzMW0kUKY9EcoaDvbMgyo0tp
sE2QnnGV5Ykue3aBtfeKtuCXeeHOHLGm2JPG14d9S6Jf5y58lxrMbsRZpw0/ISYZ
Gj0RANzyP1r10CQjuMNkzxnpW+QpjEzLrFDxjq7xkbKn8x62J4fSM2tZMlVOE9DV
1Mc45/1r3VgEdzkONSBykT51woTdcovUnP4gEg+REky1Wb1S1rk8m1MRAIq4T1Yu
cRyqpNNYhJbXofPwNMhrdo9fqhaCYTrxf8ZpiFDnZHqF28zQtSUm0YgFJR4vZkAd
esBWo++FVefIL3T6VkbOHKN4I4dk+EWlERHjVQz+uQKBgQDrebFo6qoAqr1O/c7d
CDTU4FXZcml7IPSLL3U196WB/MfsP+UVzgD4+DOHenQwHbbj7ta/rgF7iIHliqBX
WdGFgywPs7sNhq11av5ZEAXHD7r8eZBjKlV8IsvMA51MCp1/SK8McxVhBVEJgsGL
VSRxvRz9tVR7wlKcg7DE0aBF3wKBgQDiDUjUNU2HDDmYuCuEmsjH/c6f/P+BjLXp
LnKW0aUbvQl/nDTMTJTIu0zG0+OJhL4GWDkB9DW115kxCGFmZMvrk3LeDqg1QWDQ
d3cxgEdSSsRWBsiABvIn7Fno/MN2NrZd8Wdfk7HIIF0rGOy9ja5/PVl0FxUt4O1X
dRmQ3oq41QKBgHoD4djyl8qmrleLDrDburx/zhxRu7SQnAavPbYML9fOSy3w4dzN
lRVtTw4pdqEkFIvBS8eg+6WuU1jE31bD9NyQ3rj4MbnNin4oRcmSktvWG9cNirLH
0en0AdQiH1Syv2+gEwyJaY+PeLFL7swq/ypsiuQwHKnQRIxTdLpXwQvTAoGAS7+Z
3QpzjUKKdmOYqZnYmDOzrqbv07CcMKRQ37smsbHZ4fotMxyiatVgt+u+/pENwECF
8eKssN+rROQDB3XVY36IamLM+POMhq7RsTPEMo49Vnp1a3loYfpwcoNo2E8jMz22
ny91zpMRxWRXyHkWtSqQtDcb8MDDp5/kzkfUgnUCgYEAv8CVWPKTuw83/nnqZg26
URXJ/C7hN/1uU21BuyCTMV/fLiSAsV0ucDV2spqCl3VAXcsECavERVppluVylBcR
DFa6BZS0N0x374JRidFWV0a+Mz7pTqC0TO/M3+y6yaDd766J3bkdh2sq8pnhAnXc
qPYXB5U6tdTrexzaYBKr4gQ=
-----END PRIVATE KEY-----`

	t.Run("TestParseRSAPrivateKey", func(t *testing.T) {
		parsed, err := getRSAPrivateKeyFromString(rsaPrivateKey)
		if err != nil {
			t.Fatalf("failed to parse PEM cert: %v", err)
		}

		if _, err := jwk.New(parsed); err != nil {
			t.Errorf("RSA private key failed when it was expected to succeed, got %v", err)
		}
	})

	t.Run("TestParseRSAPrivateKeyBase64", func(t *testing.T) {
		b64 := base64.StdEncoding.EncodeToString([]byte(rsaPrivateKey))

		parsed, err := getRSAPrivateKeyFromString(b64)
		if err != nil {
			t.Fatalf("failed to parse PEM cert: %v", err)
		}

		if _, err := jwk.New(parsed); err != nil {
			t.Errorf("RSA private key (base64) failed when it was expected to succeed, got %v", err)
		}
	})

	t.Run("TestParseRSAPrivateKeyPKCS8", func(t *testing.T) {
		parsed, err := getRSAPrivateKeyFromString(rsaPrivateKeyPKCS8)
		if err != nil {
			t.Fatalf("failed to parse PEM cert: %v", err)
		}

		if _, err := jwk.New(parsed); err != nil {
			t.Errorf("RSA private key (PKCS8) failed when it was expected to succeed, got %v", err)
		}
	})

}
