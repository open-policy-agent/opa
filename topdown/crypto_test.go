package topdown

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

var rootCA = `-----BEGIN CERTIFICATE-----
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
var intermediateCA = `-----BEGIN CERTIFICATE-----
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
var leaf = `-----BEGIN CERTIFICATE-----
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

var rsaPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
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

var rsaPrivateKeyPKCS8 = `-----BEGIN PRIVATE KEY-----
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

var keyPemEC = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`

var keyEd25519 = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIJHG93jlLLLTF6Stky5+8Q7mMpgCkYYTO12NDAzlJn3w
-----END PRIVATE KEY-----
`

var partiallyValidPEMString = `
something else
-----BEGIN PRIVATE KEY-----
 MC4CAQAwBQYDK2VwBCIEIJHG93jlLLLTF6Stky5+8Q7mMpgCkYYTO12NDAzlJn3w
-----END PRIVATE KEY-----
something else
-----BEGIN CERTIFICATE-----
MIIBcDCCARagAwIBAgIJAMZmuGSIfvgzMAoGCCqGSM49BAMCMBMxETAPBgNVBAMM
CHdoYXRldmVyMB4XDTE4MDgxMDE0Mjg1NFoXDTE4MDkwOTE0Mjg1NFowEzERMA8G
A1UEAwwId2hhdGV2ZXIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATPwn3WCEXL
mjp/bFniDwuwsfu7bASlPae2PyWhqGeWwe23Xlyx+tSqxlkXYe4pZ23BkAAscpGj
yn5gXHExyDlKo1MwUTAdBgNVHQ4EFgQUElRjSoVgKjUqY5AXz2o74cLzzS8wHwYD
VR0jBBgwFoAUElRjSoVgKjUqY5AXz2o74cLzzS8wDwYDVR0TAQH/BAUwAwEB/zAK
BggqhkjOPQQDAgNIADBFAiEA4yQ/88ZrUX68c6kOe9G11u8NUaUzd8pLOtkKhniN
OHoCIHmNX37JOqTcTzGn2u9+c8NlnvZ0uDvsd1BmKPaUmjmm
-----END CERTIFICATE-----
something else
-----BEGIN PRIVATE KEY-----
 MC4CAQAwBQYDK2VwBCIEIJHG93jlLLLTF6Stky5+8Q7mMpgCkYYTO12NDAzlJn3w
-----END PRIVATE KEY-----
something else
`
var invalidData = `nothingtoseehere`

func TestX509ParseAndVerify(t *testing.T) {

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

func Test_parsex509KeyPair(t *testing.T) {
	certPemEC := []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
	keyPemEC := []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`)

	certPemx509 := []byte(`-----BEGIN CERTIFICATE-----
MIIF7zCCA9egAwIBAgIUdRHA+B0/ZgknKPlB2fswE98lzAcwDQYJKoZIhvcNAQEL
BQAwgYYxCzAJBgNVBAYTAlhYMRIwEAYDVQQIDAlTdGF0ZU5hbWUxETAPBgNVBAcM
CENpdHlOYW1lMRQwEgYDVQQKDAtDb21wYW55TmFtZTEbMBkGA1UECwwSQ29tcGFu
eVNlY3Rpb25OYW1lMR0wGwYDVQQDDBRDb21tb25OYW1lT3JIb3N0bmFtZTAeFw0y
MzA1MTAxMjUxMjhaFw0zMzA1MDcxMjUxMjhaMIGGMQswCQYDVQQGEwJYWDESMBAG
A1UECAwJU3RhdGVOYW1lMREwDwYDVQQHDAhDaXR5TmFtZTEUMBIGA1UECgwLQ29t
cGFueU5hbWUxGzAZBgNVBAsMEkNvbXBhbnlTZWN0aW9uTmFtZTEdMBsGA1UEAwwU
Q29tbW9uTmFtZU9ySG9zdG5hbWUwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIK
AoICAQC3N9jvicmKpGd1P3pXeA+MoiddNjJ5vM13KqaTc129mcmHHtIrjGnhoLrN
SLWcLGKNBbsBNi5loq41sJogymNxUsxQQErXn5DiEnEQf1Bq+nSGzIdGmbmJVWqH
t4tSIHdCIEaxHDHWkIoklR4t8CgNIvhKqlGGJlvLfPVZgWHjs9HA+6+G35tp/ZgA
vSA/Nutk5P9ALOjlysiySiX6maRPk+yrNfpqzA9clIeVQQCTSGHRPeOxg1SSqpHh
MukuY01JHwotBUQlNKuDWBwz13FOuaPc74g0psNQWeCIi7IZ6cPZWJfbCkRHB0eF
paOGcnyFFafAfNYZ1wZ87H6U3U4zwr0ILxYz6Dn8B4fIkkoY19PHEmn4trHm5wbR
nJVPieffAZzqK/zcaiBFVDdGQC/Ty3K5z9+VFbIzPWIR5S6weWtbA8h3aa/E/tHS
CYWvi+4RwqPxfWQ6JevxdWC08Rgy1IPz7vqkvD0bg7Rf01vIUohO3vboH7Jz2K0o
vEGiRdcPQfBxy/R6O4estPpTLE9CSSkcTAQcifCpn3OU8/vuRLDEy9At+mad+G5V
KiSno8D5ygljOHEQICZO0cENF5uVcrNNhQYS0Jd7F9eVCSO8HUCslMqF2ednFCpS
frYIQksRTgej4bXUxlTF3vfjEZn5idVsIgj2tVQoN0drn8MASQIDAQABo1MwUTAd
BgNVHQ4EFgQUlDXyP2Hr+VQM1aHlzcN9bDlMbJswHwYDVR0jBBgwFoAUlDXyP2Hr
+VQM1aHlzcN9bDlMbJswDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AgEACzGrfXVYhfCmtFZN0Y6NKQ7g1HVGIX5V3Al4U+oo3y+9n4KVGkgV9jh/dZ8k
mzWmpXOGRfD1ufRcD9qoXy+GHRw/0bm2wkVcMvEsnMx+DkuhiVOIc0PYDNj87uK3
vNFBM39uo1fiJ9yd9SKQsjB94lVMaF1089wevPYl5qmboku2qurX3uw5VvysHAAw
U2y0x8y/c3jw9MfK1X0GYhScZ0pV3y3xtlqrHnNjSRaZ3pfPl74ac0bgtE9q8Jt5
o3D0gfZi08RkunnkgJ1/Pd5iaH/XGgTnz5wSEAzTRBXHKYYoFpFWL0rz0kJojfCw
Rut6Omn/L6nQW/R5zwsqvkADuo/KXY2N69/RxPw0z8Eq6YMTt8pW8e5dzRFQTKcv
KTt2B50uTUDRPQmovvF3FdjrYZbHrFXGQr2hezl56BDRJBZO8HCH44B1swz+t2If
ouv2PHd05WvCTr/3GE/fbTjZlUVx8Za2TYwVjHSS9w9kQCUvw3W3nNuQzNIGTbsa
DvT3ySt22dD40MYk4zNsz8wV0QV0vbyzz+1FTIreVWIIxi0uoygMhuFL7Izn1xsA
T8Mellt37m03evBLpQwOXqt7RWZSZ2YruT6rZdgdA9te+f7ULaaCFD2qc9hDTK8j
zhw2N5pcH1ZXdfAF3MQj5Va+HICDIZ/pmNcdVRgYlV7Hd9g=
-----END CERTIFICATE-----
`)

	certPemRSA := []byte(`-----BEGIN PRIVATE KEY-----
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
1rJjQEsiZxH+QPR88tuByUVG000lpA==
-----END PRIVATE KEY-----
`)

	certDERx509 := []byte(`MIIF7zCCA9egAwIBAgIUdRHA+B0/ZgknKPlB2fswE98lzAcwDQYJKoZIhvcNAQELBQAwgYYxCzAJ
BgNVBAYTAlhYMRIwEAYDVQQIDAlTdGF0ZU5hbWUxETAPBgNVBAcMCENpdHlOYW1lMRQwEgYDVQQK
DAtDb21wYW55TmFtZTEbMBkGA1UECwwSQ29tcGFueVNlY3Rpb25OYW1lMR0wGwYDVQQDDBRDb21t
b25OYW1lT3JIb3N0bmFtZTAeFw0yMzA1MTAxMjUxMjhaFw0zMzA1MDcxMjUxMjhaMIGGMQswCQYD
VQQGEwJYWDESMBAGA1UECAwJU3RhdGVOYW1lMREwDwYDVQQHDAhDaXR5TmFtZTEUMBIGA1UECgwL
Q29tcGFueU5hbWUxGzAZBgNVBAsMEkNvbXBhbnlTZWN0aW9uTmFtZTEdMBsGA1UEAwwUQ29tbW9u
TmFtZU9ySG9zdG5hbWUwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQC3N9jvicmKpGd1
P3pXeA+MoiddNjJ5vM13KqaTc129mcmHHtIrjGnhoLrNSLWcLGKNBbsBNi5loq41sJogymNxUsxQ
QErXn5DiEnEQf1Bq+nSGzIdGmbmJVWqHt4tSIHdCIEaxHDHWkIoklR4t8CgNIvhKqlGGJlvLfPVZ
gWHjs9HA+6+G35tp/ZgAvSA/Nutk5P9ALOjlysiySiX6maRPk+yrNfpqzA9clIeVQQCTSGHRPeOx
g1SSqpHhMukuY01JHwotBUQlNKuDWBwz13FOuaPc74g0psNQWeCIi7IZ6cPZWJfbCkRHB0eFpaOG
cnyFFafAfNYZ1wZ87H6U3U4zwr0ILxYz6Dn8B4fIkkoY19PHEmn4trHm5wbRnJVPieffAZzqK/zc
aiBFVDdGQC/Ty3K5z9+VFbIzPWIR5S6weWtbA8h3aa/E/tHSCYWvi+4RwqPxfWQ6JevxdWC08Rgy
1IPz7vqkvD0bg7Rf01vIUohO3vboH7Jz2K0ovEGiRdcPQfBxy/R6O4estPpTLE9CSSkcTAQcifCp
n3OU8/vuRLDEy9At+mad+G5VKiSno8D5ygljOHEQICZO0cENF5uVcrNNhQYS0Jd7F9eVCSO8HUCs
lMqF2ednFCpSfrYIQksRTgej4bXUxlTF3vfjEZn5idVsIgj2tVQoN0drn8MASQIDAQABo1MwUTAd
BgNVHQ4EFgQUlDXyP2Hr+VQM1aHlzcN9bDlMbJswHwYDVR0jBBgwFoAUlDXyP2Hr+VQM1aHlzcN9
bDlMbJswDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEACzGrfXVYhfCmtFZN0Y6N
KQ7g1HVGIX5V3Al4U+oo3y+9n4KVGkgV9jh/dZ8kmzWmpXOGRfD1ufRcD9qoXy+GHRw/0bm2wkVc
MvEsnMx+DkuhiVOIc0PYDNj87uK3vNFBM39uo1fiJ9yd9SKQsjB94lVMaF1089wevPYl5qmboku2
qurX3uw5VvysHAAwU2y0x8y/c3jw9MfK1X0GYhScZ0pV3y3xtlqrHnNjSRaZ3pfPl74ac0bgtE9q
8Jt5o3D0gfZi08RkunnkgJ1/Pd5iaH/XGgTnz5wSEAzTRBXHKYYoFpFWL0rz0kJojfCwRut6Omn/
L6nQW/R5zwsqvkADuo/KXY2N69/RxPw0z8Eq6YMTt8pW8e5dzRFQTKcvKTt2B50uTUDRPQmovvF3
FdjrYZbHrFXGQr2hezl56BDRJBZO8HCH44B1swz+t2Ifouv2PHd05WvCTr/3GE/fbTjZlUVx8Za2
TYwVjHSS9w9kQCUvw3W3nNuQzNIGTbsaDvT3ySt22dD40MYk4zNsz8wV0QV0vbyzz+1FTIreVWII
xi0uoygMhuFL7Izn1xsAT8Mellt37m03evBLpQwOXqt7RWZSZ2YruT6rZdgdA9te+f7ULaaCFD2q
c9hDTK8jzhw2N5pcH1ZXdfAF3MQj5Va+HICDIZ/pmNcdVRgYlV7Hd9g=
`)
	certDERRSA := []byte(`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
`)

	certPemRSACrtB64 := []byte(`LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUY3ekNDQTllZ0F3SUJBZ0lVZFJIQStCMC9aZ2tuS1BsQjJmc3dFOThsekFjd0RRWUpLb1pJaHZjTkFRRUwKQlFBd2dZWXhDekFKQmdOVkJBWVRBbGhZTVJJd0VBWURWUVFJREFsVGRHRjBaVTVoYldVeEVUQVBCZ05WQkFjTQpDRU5wZEhsT1lXMWxNUlF3RWdZRFZRUUtEQXREYjIxd1lXNTVUbUZ0WlRFYk1Ca0dBMVVFQ3d3U1EyOXRjR0Z1CmVWTmxZM1JwYjI1T1lXMWxNUjB3R3dZRFZRUUREQlJEYjIxdGIyNU9ZVzFsVDNKSWIzTjBibUZ0WlRBZUZ3MHkKTXpBMU1UQXhNalV4TWpoYUZ3MHpNekExTURjeE1qVXhNamhhTUlHR01Rc3dDUVlEVlFRR0V3SllXREVTTUJBRwpBMVVFQ0F3SlUzUmhkR1ZPWVcxbE1SRXdEd1lEVlFRSERBaERhWFI1VG1GdFpURVVNQklHQTFVRUNnd0xRMjl0CmNHRnVlVTVoYldVeEd6QVpCZ05WQkFzTUVrTnZiWEJoYm5sVFpXTjBhVzl1VG1GdFpURWRNQnNHQTFVRUF3d1UKUTI5dGJXOXVUbUZ0WlU5eVNHOXpkRzVoYldVd2dnSWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUNEd0F3Z2dJSwpBb0lDQVFDM045anZpY21LcEdkMVAzcFhlQStNb2lkZE5qSjV2TTEzS3FhVGMxMjltY21ISHRJcmpHbmhvTHJOClNMV2NMR0tOQmJzQk5pNWxvcTQxc0pvZ3ltTnhVc3hRUUVyWG41RGlFbkVRZjFCcStuU0d6SWRHbWJtSlZXcUgKdDR0U0lIZENJRWF4SERIV2tJb2tsUjR0OENnTkl2aEtxbEdHSmx2TGZQVlpnV0hqczlIQSs2K0czNXRwL1pnQQp2U0EvTnV0azVQOUFMT2pseXNpeVNpWDZtYVJQayt5ck5mcHF6QTljbEllVlFRQ1RTR0hSUGVPeGcxU1NxcEhoCk11a3VZMDFKSHdvdEJVUWxOS3VEV0J3ejEzRk91YVBjNzRnMHBzTlFXZUNJaTdJWjZjUFpXSmZiQ2tSSEIwZUYKcGFPR2NueUZGYWZBZk5ZWjF3Wjg3SDZVM1U0endyMElMeFl6NkRuOEI0Zklra29ZMTlQSEVtbjR0ckhtNXdiUgpuSlZQaWVmZkFaenFLL3pjYWlCRlZEZEdRQy9UeTNLNXo5K1ZGYkl6UFdJUjVTNndlV3RiQThoM2FhL0UvdEhTCkNZV3ZpKzRSd3FQeGZXUTZKZXZ4ZFdDMDhSZ3kxSVB6N3Zxa3ZEMGJnN1JmMDF2SVVvaE8zdmJvSDdKejJLMG8KdkVHaVJkY1BRZkJ4eS9SNk80ZXN0UHBUTEU5Q1NTa2NUQVFjaWZDcG4zT1U4L3Z1UkxERXk5QXQrbWFkK0c1VgpLaVNubzhENXlnbGpPSEVRSUNaTzBjRU5GNXVWY3JOTmhRWVMwSmQ3RjllVkNTTzhIVUNzbE1xRjJlZG5GQ3BTCmZyWUlRa3NSVGdlajRiWFV4bFRGM3ZmakVabjVpZFZzSWdqMnRWUW9OMGRybjhNQVNRSURBUUFCbzFNd1VUQWQKQmdOVkhRNEVGZ1FVbERYeVAySHIrVlFNMWFIbHpjTjliRGxNYkpzd0h3WURWUjBqQkJnd0ZvQVVsRFh5UDJIcgorVlFNMWFIbHpjTjliRGxNYkpzd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBTkJna3Foa2lHOXcwQkFRc0ZBQU9DCkFnRUFDekdyZlhWWWhmQ210RlpOMFk2TktRN2cxSFZHSVg1VjNBbDRVK29vM3krOW40S1ZHa2dWOWpoL2RaOGsKbXpXbXBYT0dSZkQxdWZSY0Q5cW9YeStHSFJ3LzBibTJ3a1ZjTXZFc25NeCtEa3VoaVZPSWMwUFlETmo4N3VLMwp2TkZCTTM5dW8xZmlKOXlkOVNLUXNqQjk0bFZNYUYxMDg5d2V2UFlsNXFtYm9rdTJxdXJYM3V3NVZ2eXNIQUF3ClUyeTB4OHkvYzNqdzlNZksxWDBHWWhTY1owcFYzeTN4dGxxckhuTmpTUmFaM3BmUGw3NGFjMGJndEU5cThKdDUKbzNEMGdmWmkwOFJrdW5ua2dKMS9QZDVpYUgvWEdnVG56NXdTRUF6VFJCWEhLWVlvRnBGV0wwcnowa0pvamZDdwpSdXQ2T21uL0w2blFXL1I1endzcXZrQUR1by9LWFkyTjY5L1J4UHcwejhFcTZZTVR0OHBXOGU1ZHpSRlFUS2N2CktUdDJCNTB1VFVEUlBRbW92dkYzRmRqcllaYkhyRlhHUXIyaGV6bDU2QkRSSkJaTzhIQ0g0NEIxc3d6K3QySWYKb3V2MlBIZDA1V3ZDVHIvM0dFL2ZiVGpabFVWeDhaYTJUWXdWakhTUzl3OWtRQ1V2dzNXM25OdVF6TklHVGJzYQpEdlQzeVN0MjJkRDQwTVlrNHpOc3o4d1YwUVYwdmJ5enorMUZUSXJlVldJSXhpMHVveWdNaHVGTDdJem4xeHNBClQ4TWVsbHQzN20wM2V2QkxwUXdPWHF0N1JXWlNaMllydVQ2clpkZ2RBOXRlK2Y3VUxhYUNGRDJxYzloRFRLOGoKemh3Mk41cGNIMVpYZGZBRjNNUWo1VmErSElDRElaL3BtTmNkVlJnWWxWN0hkOWc9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K`)
	certPemRSAKeyB64 := []byte(`LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUpRZ0lCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQ1N3d2dna29BZ0VBQW9JQ0FRQzNOOWp2aWNtS3BHZDEKUDNwWGVBK01vaWRkTmpKNXZNMTNLcWFUYzEyOW1jbUhIdElyakduaG9Mck5TTFdjTEdLTkJic0JOaTVsb3E0MQpzSm9neW1OeFVzeFFRRXJYbjVEaUVuRVFmMUJxK25TR3pJZEdtYm1KVldxSHQ0dFNJSGRDSUVheEhESFdrSW9rCmxSNHQ4Q2dOSXZoS3FsR0dKbHZMZlBWWmdXSGpzOUhBKzYrRzM1dHAvWmdBdlNBL051dGs1UDlBTE9qbHlzaXkKU2lYNm1hUlBrK3lyTmZwcXpBOWNsSWVWUVFDVFNHSFJQZU94ZzFTU3FwSGhNdWt1WTAxSkh3b3RCVVFsTkt1RApXQnd6MTNGT3VhUGM3NGcwcHNOUVdlQ0lpN0laNmNQWldKZmJDa1JIQjBlRnBhT0djbnlGRmFmQWZOWVoxd1o4CjdINlUzVTR6d3IwSUx4WXo2RG44QjRmSWtrb1kxOVBIRW1uNHRySG01d2JSbkpWUGllZmZBWnpxSy96Y2FpQkYKVkRkR1FDL1R5M0s1ejkrVkZiSXpQV0lSNVM2d2VXdGJBOGgzYWEvRS90SFNDWVd2aSs0UndxUHhmV1E2SmV2eApkV0MwOFJneTFJUHo3dnFrdkQwYmc3UmYwMXZJVW9oTzN2Ym9IN0p6Mkswb3ZFR2lSZGNQUWZCeHkvUjZPNGVzCnRQcFRMRTlDU1NrY1RBUWNpZkNwbjNPVTgvdnVSTERFeTlBdCttYWQrRzVWS2lTbm84RDV5Z2xqT0hFUUlDWk8KMGNFTkY1dVZjck5OaFFZUzBKZDdGOWVWQ1NPOEhVQ3NsTXFGMmVkbkZDcFNmcllJUWtzUlRnZWo0YlhVeGxURgozdmZqRVpuNWlkVnNJZ2oydFZRb04wZHJuOE1BU1FJREFRQUJBb0lDQUJKaUttUm9neDRqM2xTYm5Lc3Jtd1hOCm5GMEVLOTdlcEpBTktiOFlQNExmYkNMZ1l3Nm5EVldzQXFwSDNoOFFMZ2cvMTdKb3JSR2FGOWcvd3N0QSsyYmEKdTdEZXJwUEJpVEJCMFBIcWtGZFhqM3NhQ1FXNnRXelQ4dmN3b2F4SklTWXpwbHd0ZTh1dlg0a0pwRWhRTlRpUwpObThKZFZvY1BiQW1kdGkyL0dzME52cmgxZ3dXb2htcHJnZCs4bjRkUk5Pd0RYTnpQaUFXYjNwQ0tkcmg4U1JoCjc0aURSei9SZjBZWENoNmQ4ZENWWGVrNGlFRGVzRXp5QythWWJPQ3dhb2dJY3dVdTV0WkQyV1M1b2NUSzNHM2QKZnhWVFBHdXFBdVZzU3pUd0xWdmZ3bnlyb0xzRDVmTnBoZEhoVzQzSkxYak9BakcwWk9nZFZPT1NlQ1g0S1prbwptcWc4b3Y1TksxclVRVGQwNng2bmg0elBXdEdaTDIwbE9EMTZvRWgzUlZoRFU5akQ1U25aZ2M1Mzg2bVkxeTVJClV0QTBUUnh3MzNIcjRWT1ExaXVCZzlrNTU2MzFIcGl0clBWZ1QvLzVzYzdjMjhpQ2lBRnVFZGg3SDF5cG5NNkgKc1pjL2prRTJBOFF1NnpKSTV2NzRoYkFaTmxONVMrcDU4ajhrV2pPYTd4bnZwYk5saXc1a2JDK2Ezb25ZSVNHZApTNVZITzVEVkVaOEY3aUtUTS8zTEFZdDZScG0vdEZJOUhFZVRGWHVCVEVsYTlsb0srUWdXbjF1QVVMeGtKUHFyCkFXZ0tzL0FYYWpxVyt0d3ZUM2M5cUFDaGFTakhjV3lxem05WGh6RDRwNTg2cnpDZ3M5VUNSSStudlEvOGFIMWoKNm9CdkRqbUxpem9WUEVkR2k1VkJBb0lCQVFEM2pRZThEdVIxVjYveS9HcXRNRVcyVXptdTl2d1R4VXRBSFZtawpob25PbFo0MDVEL1B3b0s1dXBORjV2QWtxNDN1b1M2OXVuQWxxbGpraVNmOTEzU3pYZEE4dXRRYlQzVFczUmxWCkZ2dXZlN3YvcWtRZFdScm5rdXJhcWtFbnhUMnlKOGdlaG1Qcmo1STMwVHVRVXpSdmFRc2hyYkIwV3NCcGcrRmIKdWJnV29wU21hVTY5aEJpSGxwWFYrcjJRUXIrVllHT3JLUkNSWDRhMGxtbEJxbDdnemVZSTA2d2RpamszelF5cgorcVltYjRkdEFqd3huQ09CRG9qbVJBM2hGTmpVbC8wNlVvMmZzUEM1cjduWlA2cURUMUU2MU5tVDhPZ3kyUWE1Cm90Uk4wS1E3bThGRXAwUDA4aHByOTRxVlZLaHB2NzMzcGMwQkhCQXZ6VG9RdTdKcEFvSUJBUUM5ZUxZZzVSS3cKMDZQSEVmRlhMTnZ1NU12Q2I0T2NHY1BHUElLaHEyWExTbFJSSElVSkdueFB2RVRrcm93YXhDVGVVc2g0WUFtUApZUjZxKzE2aHRwN1JBZ1RWNkxvZUpnQ2hDNVRvTWlkZkR2MmxEUnZneVpMWWtsU0VIcEpaSXRWWTI4NzFWTkRaClhFQWwwOGFGcHlENDNseXdNc3UxaHYzSnRMVGJVTDNZdGtBZnBQVFJBZ0ZaaFlxbVJUdTJKeXN5dXVoWWFVbS8KeEw3WDcvYmlJZ2FPemR1YkF0QXBFTUlEbnFOdVdKOUVBQit4a1c5VUIyTFpRZjBuOTh2bG40aUowaDhsb1V5RAphWElPbWpDZFJXeGFzeVRRQkNEZUR0eXgxbHNvb1R1U2JWU1FFSHVUeUgzVno2UHZBZC9xNGxwRW5hY0UvMjE5CjRXUEh6NnArcDJMaEFvSUJBQ28yQXhhZkYzZW16eHJJemN2Z1NsTFBtQ3RzZEFsUEFBamJ1RmhrbElVRVlDaTIKcnViWFRRRXNma1pTSGFxekVnMlpzR1dycjhuTVpVSDYzVFhja2txdmVYMlJnZTl5T2dNVlNtZUc5cjJ5aEprUQp5SEtVcWhESXJZRkJ2TUJ5VXBYWlVMZGJ4UmY2c0QwU1VXekhzMDQ0QkN6bStBcXZHdFlqSmI5RlNNMmJSV3VtCjAwVmZpK3M2MHl2Y2lJeGJ4VjFNUlZKL094TCt6ZkpuSDJXU0RvR1l1bHZROUMxSlQzNWpXWUROeVowT01YSjIKQ2h1UGUwSmJYeDZjaGgxV042N3doNzUxS3k4S3RkR0QxRlhtRkVZMXRTMHA5RHZVdlZOR1RHNUZCSnlNTWlUego1eDIwdzlLMW9hbTlXUVVqbldBQzBQcTBhK04vakljS0lKZVAyZGtDZ2dFQVhPZzhKcFV0UFJnS1R5czFOSklDCnBubjZrRFV1Uy9VMlVwYUpWODA3OVJ0VmpSQjNDNmU1SFVBc2FCWlBEVER4QXpPRXFjSXQ3ZWlwcVIzcG9WSnoKUGZuSGRUelJSc2RMdDZ4K0wvMm40S3p4STJYeUxaK3FLaGhXNlJJMG9SQzduUDdyMU5EcU9DdE1LVUJYTUdKcgpnSjFJeGYyaWRqamphV3o2NGpBTlo1NjJnczNZWGtTbGRNaE8zSWxHWm1OK2d6bXpoT2JjQ3ZUbXYrd2pHMitqCjE1S0tCTkMwVWU2dHRDaXQ2d1g1MHRaY3RDMmtjWWZOcU1yNjRBWmFMUmExVlI5N3RuQUpuTWF2N3drY25ZSFYKU0FSZ0lNQmxmWDI4S2xmNkMwcEVjK0M0Zm93V2pMamJPMlM5OWd6dFI3Z0dtMjdTMzFpQTBDRWRWSFU0SFRMbgpBUUtDQVFFQXNzajBmRThqa0ZpdnpRMmxTZTBEeGhwUXFHZVF1UGROYkJKVnNBcVdmRXRKQ0h2R0JYaTZBeEIrClFnN015RE14dnVPYXQ1TVNUQ01GYzNYdVFUdEtxdlF3SGxXMXZpTnJxckNzRHVRNTVDa1ZQTUxmazk4VlFoeHkKMnpUd20zOUd5a1ZpLy9CaU5LQzRCU2ZtZlN6cGdiYlJnSEI2Ny9zcGIwejBMbXUrWG5uL0ROemxxS1pvQUZFQgpwcDdzNkVmWGdJN1Npbm94dGxuRHM4eDdFM2dlcEtENVVWbnQzM3FZbUhCOVZ3WHpUZDBaSkhrY0x1b1VpanJjCko2cENYVTcrRlpPd3pJQi9IcFFPZ2pwdklqQ0pJaHRGeFZ5Z2JqdXVjRThRS2Naaml5VXNGTHZkQzVkZTFNZVQKMXJKalFFc2laeEgrUVBSODh0dUJ5VVZHMDAwbHBBPT0KLS0tLS1FTkQgUFJJVkFURSBLRVktLS0tLQo=`)

	t.Run("ParseX509KeyPairEC", func(t *testing.T) {
		validKeyPairEc, err := getTLSx509KeyPairFromString(certPemEC, keyPemEC)
		if err != nil {
			t.Fatal("failed to parse x509 key pair")
		}
		if validKeyPairEc == nil {
			t.Fatal("expected certificate but got nil")
		}

	})

	t.Run("ParseX509KeyPairRSA", func(t *testing.T) {
		validCertPair, err := getTLSx509KeyPairFromString(certPemx509, certPemRSA)
		if err != nil {
			t.Fatal("failed to parse x509 Pair with RSAkey")
		}
		if validCertPair == nil {
			t.Fatal("expected certificate but got nil")
		}

	})

	t.Run("ParseX509KeyPairRSABase64", func(t *testing.T) {
		validCertPair, err := getTLSx509KeyPairFromString(certPemRSACrtB64, certPemRSAKeyB64)
		if err != nil {
			t.Fatal("failed to parse x509 Pair with RSAkey")
		}
		if validCertPair == nil {
			t.Fatal("expected certificate but got nil")
		}

	})

	t.Run("ParseX509KeyPairDERx509", func(t *testing.T) {
		validCertPair, err := getTLSx509KeyPairFromString(certDERx509, certDERRSA)
		if err != nil {
			t.Fatal("failed to parse x509 Pair with RSAkey", err)
		}
		if validCertPair == nil {
			t.Fatal("expected certificate but got nil")
		}
	})

	t.Run("ParseX509KeyPairPEMstringB64Key", func(t *testing.T) {
		validCertPair, err := getTLSx509KeyPairFromString(certPemx509, certPemRSAKeyB64)
		if err != nil {
			t.Fatal("failed to parse x509 Pair with RSAkey")
		}
		if validCertPair == nil {
			t.Fatal("expected certificate but got nil")
		}

	})

	t.Run("ParseX509KeyPairB64CRTBPEMKey", func(t *testing.T) {
		validCertPair, err := getTLSx509KeyPairFromString(certPemRSACrtB64, certPemRSA)
		if err != nil {
			t.Fatal("failed to parse x509 Pair with RSAkey")
		}
		if validCertPair == nil {
			t.Fatal("expected certificate but got nil")
		}

	})

	t.Run("ParseX509KeyPairMisMatchedTypes", func(t *testing.T) {
		certPair, err := getTLSx509KeyPairFromString(certPemEC, certPemRSA)
		if err == nil {
			t.Fatal("expected error but got nil")
		}
		if certPair != nil {
			t.Fatalf("expected no certificate but got %v\n", certPair)
		}
	})

}

func Test_getPrivateKeyFromPEMData(t *testing.T) {
	tests := map[string]struct {
		input    string
		wantErr  string
		keyCheck func(t *testing.T, keys []crypto.PrivateKey)
	}{
		"invalid data": {
			input: invalidData,
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 0 {
					t.Fatalf("expected no keys but got %d", len(keys))
				}
			},
		},
		"rsa key": {
			input: rsaPrivateKey,
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*rsa.PrivateKey); !ok {
					t.Fatalf("expected rsa key but got %T", keys[0])
				}
			},
		},
		"base64 rsa key": {
			input: base64.StdEncoding.EncodeToString([]byte(rsaPrivateKey)),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*rsa.PrivateKey); !ok {
					t.Fatalf("expected rsa key but got %T", keys[0])
				}
			},
		},
		"rsa key pkcs8": {
			input: rsaPrivateKeyPKCS8,
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*rsa.PrivateKey); !ok {
					t.Fatalf("expected rsa key but got %T", keys[0])
				}
			},
		},
		"base64 rsa key pkcs8": {
			input: base64.StdEncoding.EncodeToString([]byte(rsaPrivateKeyPKCS8)),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*rsa.PrivateKey); !ok {
					t.Fatalf("expected rsa key but got %T", keys[0])
				}
			},
		},
		"ec key": {
			input: keyPemEC,
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*ecdsa.PrivateKey); !ok {
					t.Fatalf("expected ecdsa key but got %T", keys[0])
				}
			},
		},
		"base64 ec key": {
			input: base64.StdEncoding.EncodeToString([]byte(keyPemEC)),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*ecdsa.PrivateKey); !ok {
					t.Fatalf("expected ecdsa key but got %T", keys[0])
				}
			},
		},
		"ed key": {
			input: keyEd25519,
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(ed25519.PrivateKey); !ok {
					t.Fatalf("expected ed25519 key but got %T", keys[0])
				}
			},
		},
		"base64 ed key": {
			input: base64.StdEncoding.EncodeToString([]byte(keyEd25519)),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(ed25519.PrivateKey); !ok {
					t.Fatalf("expected ed25519 key but got %T", keys[0])
				}
			},
		},
		"other PEM data, no keys": {
			input: fmt.Sprintf("%s\n%s\n%s\n", rootCA, intermediateCA, leaf),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 0 {
					t.Fatalf("expected no keys but got %d", len(keys))
				}
			},
		},
		"partially valid PEM data": {
			input: partiallyValidPEMString,
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 2 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(ed25519.PrivateKey); !ok {
					t.Fatalf("expected ed25519 key but got %T", keys[0])
				}
				if _, ok := keys[1].(ed25519.PrivateKey); !ok {
					t.Fatalf("expected ed25519 key but got %T", keys[0])
				}
			},
		},
		"mixed PEM data": {
			input: fmt.Sprintf("%s\n%s\n%s\n%s", rootCA, intermediateCA, leaf, keyPemEC),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 1 {
					t.Fatalf("expected 1 key but got %d", len(keys))
				}
				if _, ok := keys[0].(*ecdsa.PrivateKey); !ok {
					t.Fatalf("expected ecdsa key but got %T", keys[0])
				}
			},
		},
		"mixed PEM data, two keys": {
			input: fmt.Sprintf("%s\n%s\n%s\n%s\n%s", rootCA, intermediateCA, leaf, keyPemEC, rsaPrivateKey),
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 2 {
					t.Fatalf("expected 2 keys but got %d", len(keys))
				}
				if _, ok := keys[0].(*ecdsa.PrivateKey); !ok {
					t.Fatalf("expected ecdsa key but got %T", keys[0])
				}
				if _, ok := keys[1].(*rsa.PrivateKey); !ok {
					t.Fatalf("expected rsa key but got %T", keys[0])
				}
			},
		},
		"corrupted key": {
			input: `-----BEGIN PRIVATE KEY-----
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
-----END PRIVATE KEY-----
`,
			wantErr: "asn1: structure error",
			keyCheck: func(t *testing.T, keys []crypto.PrivateKey) {
				if len(keys) != 0 {
					t.Fatalf("expected no keys but got %d", len(keys))
				}
			},
		},
	}
	for name, testData := range tests {
		t.Run(name, func(t *testing.T) {
			keys, err := getPrivateKeysFromPEMData(testData.input)
			if testData.wantErr != "" {
				if err != nil && !strings.Contains(err.Error(), testData.wantErr) {
					t.Fatalf("got error: %v, want error: %v", err, testData.wantErr)
				} else if err == nil {
					t.Fatalf("expected error: %v", testData.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			testData.keyCheck(t, keys)
		})
	}
}
