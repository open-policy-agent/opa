// Copyright 2017 johandorland ( https://github.com/johandorland )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gojsonschema

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type jsonSchemaTest struct {
	Description string `json:"description"`
	// Some tests may not always pass, so some tests are manually edited to include
	// an extra attribute whether that specific test should be disabled and skipped
	Disabled bool                 `json:"disabled"`
	Schema   interface{}          `json:"schema"`
	Tests    []jsonSchemaTestCase `json:"tests"`
}
type jsonSchemaTestCase struct {
	Description string      `json:"description"`
	Data        interface{} `json:"data"`
	Valid       bool        `json:"valid"`
}

// Skip any directories not named appropiately
// filepath.Walk will also visit files in the root of the test directory
var testDirectories = regexp.MustCompile(`(draft\d+)`)
var draftMapping = map[string]Draft{
	"draft4": Draft4,
	"draft6": Draft6,
	"draft7": Draft7,
}

func executeTests(t *testing.T, path string) error {
	file, err := os.Open(path)
	if err != nil {
		t.Errorf("Error (%s)\n", err.Error())
	}
	fmt.Println(file.Name())

	var tests []jsonSchemaTest
	d := json.NewDecoder(file)
	d.UseNumber()
	err = d.Decode(&tests)

	if err != nil {
		t.Errorf("Error (%s)\n", err.Error())
	}

	draft := Hybrid
	if m := testDirectories.FindString(path); m != "" {
		draft = draftMapping[m]
	}

	for _, test := range tests {
		fmt.Println("    " + test.Description)

		if test.Disabled {
			continue
		}

		testSchemaLoader := NewRawLoader(test.Schema)
		sl := NewSchemaLoader()
		sl.Draft = draft
		sl.Validate = true
		testSchema, err := sl.Compile(testSchemaLoader)

		if err != nil {
			t.Errorf("Error (%s)\n", err.Error())
		}

		for _, testCase := range test.Tests {
			testDataLoader := NewRawLoader(testCase.Data)
			result, err := testSchema.Validate(testDataLoader)

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			if result.Valid() != testCase.Valid {
				schemaString, _ := marshalToJSONString(test.Schema)
				testCaseString, _ := marshalToJSONString(testCase.Data)

				t.Errorf("Test failed : %s\n"+
					"%s.\n"+
					"%s.\n"+
					"expects: %t, given %t\n"+
					"Schema: %s\n"+
					"Data: %s\n",
					file.Name(),
					test.Description,
					testCase.Description,
					testCase.Valid,
					result.Valid(),
					*schemaString,
					*testCaseString)
			}
		}
	}
	return nil
}

func TestSuite(t *testing.T) {

	wd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	wd = filepath.Join(wd, "testdata")

	go func() {
		err := http.ListenAndServe(":1234", http.FileServer(http.Dir(filepath.Join(wd, "remotes"))))
		if err != nil {

			panic(err.Error())
		}
	}()

	SetAllowNet(nil)

	err = filepath.Walk(wd, func(path string, fileInfo os.FileInfo, err error) error {
		if fileInfo.IsDir() && path != wd && !testDirectories.MatchString(fileInfo.Name()) {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(fileInfo.Name(), ".json") {
			return nil
		}
		return executeTests(t, path)
	})
	if err != nil {
		t.Errorf("Error (%s)\n", err.Error())
	}
}

func TestFormats(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	wd = filepath.Join(wd, "testdata")

	dirs, err := os.ReadDir(wd)

	if err != nil {
		panic(err.Error())
	}

	for _, dir := range dirs {
		if testDirectories.MatchString(dir.Name()) {
			formatJSONFile := filepath.Join(wd, dir.Name(), "optional", "format.json")
			if _, err = os.Stat(formatJSONFile); err == nil {
				err = executeTests(t, formatJSONFile)
			} else {
				err = nil
			}

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			formatsDirectory := filepath.Join(wd, dir.Name(), "optional", "format")
			err = filepath.Walk(formatsDirectory, func(path string, fileInfo os.FileInfo, err error) error {
				if fileInfo == nil || !strings.HasSuffix(fileInfo.Name(), ".json") {
					return nil
				}
				return executeTests(t, path)
			})

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}
		}
	}
}

func Test_ConcurrentNetAccessModification(t *testing.T) {
	go func() {
		SetAllowNet([]string{"something"})
	}()
	SetAllowNet(nil)
}
