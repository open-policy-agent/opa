import yaml,json
paths=[
	r".\..\..\..\..\test\cases\testdata\cryptohmacmd5\test-cryptohmacmd5.yaml\cryptohmacmd5\crypto.hmac.md5_unicode",
	r".\..\..\..\..\test\cases\testdata\cryptohmacsha1\test-cryptohmacsha1.yaml\cryptohmacsha1\crypto.hmac.sha1_unicode",
	r".\..\..\..\..\test\cases\testdata\cryptohmacsha256\test-cryptohmacsha256.yaml\cryptohmacsha256\crypto.hmac.sha256_unicode",
	r".\..\..\..\..\test\cases\testdata\cryptohmacsha512\test-cryptohmacsha512.yaml\cryptohmacsha512\crypto.hmac.sha512_unicode",
	r".\..\..\..\..\test\cases\testdata\graphql\test-graphql-parse-query.yaml\graphql_parse_query\success-encoding_multibyte_characters_are_supported",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0467.yaml\jwtdecodeverify\es256-unconstrained",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0469.yaml\jwtdecodeverify\hs256-key-wrong",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0468.yaml\jwtdecodeverify\hs256-unconstrained",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0454.yaml\jwtdecodeverify\ps256-alg-ok",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0455.yaml\jwtdecodeverify\ps256-alg-wrong",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0452.yaml\jwtdecodeverify\ps256-iss-ok",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0453.yaml\jwtdecodeverify\ps256-iss-wrong",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0450.yaml\jwtdecodeverify\ps256-key-wrong",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0472.yaml\jwtdecodeverify\ps256-no-aud",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0449.yaml\jwtdecodeverify\ps256-unconstrained",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0463.yaml\jwtdecodeverify\rs256-alg-missing",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0470.yaml\jwtdecodeverify\rs256-aud",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0471.yaml\jwtdecodeverify\rs256-aud-list",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0464.yaml\jwtdecodeverify\rs256-crit-junk",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0458.yaml\jwtdecodeverify\rs256-exp-now-expired",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0459.yaml\jwtdecodeverify\rs256-exp-now-explicit-expired",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0451.yaml\jwtdecodeverify\rs256-key-wrong",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0473.yaml\jwtdecodeverify\rs256-missing-aud",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0461.yaml\jwtdecodeverify\rs256-nbf-now-ok",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0474.yaml\jwtdecodeverify\rs256-wrong-aud",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0475.yaml\jwtdecodeverify\rs256-wrong-aud-list",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0465.yaml\jwtdecodeverify\rsa256-nested",
	r".\..\..\..\..\test\cases\testdata\jwtdecodeverify\test-jwtdecodeverify-0466.yaml\jwtdecodeverify\rsa256-nested2",
	r".\..\..\..\..\test\cases\testdata\reachable\test-reachable-paths-0422.yaml\reachable_paths\multiple_paths",
	r".\..\..\..\..\test\cases\testdata\strings\test-strings-0925.yaml\strings\indexof_n_unicode_matches",
	r".\..\..\..\..\test\cases\testdata\withkeyword\test-withkeyword-1053.yaml\withkeyword\invalidate_comprehension_cache",
	r".\..\..\..\..\test\cases\testdata\withkeyword\test-withkeyword-1040.yaml\withkeyword\with_virtual_doc_exact_value",
	r".\..\..\..\..\test\cases\testdata\functions\test-functions-default.yaml\functions\default"
]
tests={}
def reader(pathFull):
    paths=pathFull.split(".yaml")
    path=paths[0]+".yaml"
    subpath=paths[1][1:].replace("\\","/")
    with open(path,"r",encoding="utf8") as test_file:
        testFile=yaml.safe_load(test_file)
    cases=(testFile["cases"])
    for case in cases:
        print(case["note"],subpath)
        if case["note"].replace(" ","_")==subpath:   
            return case
def toOpaTest(testcase):
    result=json.dumps(testcase["want_result"][0])
    if "input_term" in testcase:
        input=testcase["input_term"]
    elif "input" in testcase:
        input=json.dumps(testcase["input"])
    else: input=""
    if "data" in testcase:
        data=json.dumps(testcase["data"])
    else: data=""
    query=testcase["query"]
    policy=testcase["modules"][0]
    desc=testcase["note"]
    x="{Description: `"+desc+"`,Query:       `"+query+"`,Policy: `"+policy+"`,Data:`"+data+"`,Evals:[]Eval{{Input:`"+input+"`,Result:`{"+result.replace(": ",":").replace(", ",",")+"}`,},},},"
    return x.replace("\\\\", "\\").encode("utf-8")
out=[toOpaTest(reader(path)) for path in paths]
with open("extraTests.txt","wb+") as outFile:
    outFile.writelines(out)