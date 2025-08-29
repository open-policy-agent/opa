// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "modernc.org/sqlite"

	"github.com/docker/go-connections/nat"
	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/open-policy-agent/opa/v1/test/e2e"
)

type DBType string

const (
	Postgres DBType = "postgresql"
	MySQL    DBType = "mysql"
	MSSQL    DBType = "sqlserver"
	SQLite   DBType = "sqlite"
)

// TestConfig holds test configuration
type TestConfig struct {
	db     *sql.DB
	dbName string
	dbType DBType
	dbURL  string
}

// containerConfig holds database-specific container configuration
type containerConfig struct {
	image       string
	port        string
	env         map[string]string
	waitFor     wait.Strategy
	urlTemplate string
}

var dbConfigs = map[DBType]containerConfig{
	Postgres: {
		image: "postgres:17-alpine",
		port:  "5432/tcp",
		env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		waitFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(15 * time.Second),
		urlTemplate: "postgres://testuser:testpass@%s:%s/testdb?sslmode=disable",
	},
	MySQL: {
		image: "mysql:9",
		port:  "3306/tcp",
		env: map[string]string{
			"MYSQL_DATABASE":      "testdb",
			"MYSQL_USER":          "testuser",
			"MYSQL_PASSWORD":      "testpass",
			"MYSQL_ROOT_PASSWORD": "rootpass",
		},
		waitFor:     wait.ForLog("port: 3306  MySQL Community Server"),
		urlTemplate: "testuser:testpass@tcp(%s:%s)/testdb",
	},
	MSSQL: {
		image: "mcr.microsoft.com/mssql/server:2022-latest",
		port:  "1433/tcp",
		env: map[string]string{
			"ACCEPT_EULA":       "Y",
			"MSSQL_SA_PASSWORD": "MyStr0ngPassw0rd!",
		},
		waitFor:     wait.ForLog("Recovery is complete."),
		urlTemplate: "sqlserver://sa:MyStr0ngPassw0rd!@%s:%s",
	},
	SQLite: {
		urlTemplate: ":memory:",
	},
}

// setupTestContainer creates and starts a database container
func setupTestContainer(ctx context.Context, dbType DBType) (testcontainers.Container, string, error) {
	if dbType == SQLite {
		return nil, ":memory:", nil
	}

	config := dbConfigs[dbType]

	containerReq := testcontainers.ContainerRequest{
		Image:        config.image,
		ExposedPorts: []string{config.port},
		WaitingFor:   config.waitFor,
		Env:          config.env,
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %v", err)
	}

	port, err := container.MappedPort(ctx, nat.Port(config.port))
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container host: %v", err)
	}

	dbURL := fmt.Sprintf(config.urlTemplate, host, port.Port())
	return container, dbURL, nil
}

// getCreateTableSQL returns database-specific CREATE TABLE SQL
func getCreateTableSQL(dbType DBType) string {
	switch dbType {
	case Postgres:
		return `
        CREATE TABLE IF NOT EXISTS fruit (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            colour VARCHAR(100) NOT NULL,
            price INT
        )`
	case MySQL:
		return `
        CREATE TABLE IF NOT EXISTS fruit (
            id INT AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            colour VARCHAR(100) NOT NULL,
            price INT
        )`
	case SQLite:
		return `
        CREATE TABLE IF NOT EXISTS fruit (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name VARCHAR(100) NOT NULL,
            colour VARCHAR(100) NOT NULL,
            price INT
        )`
	case MSSQL:
		return `
        IF NOT EXISTS (SELECT * FROM sysobjects WHERE name='fruit' AND xtype='U')
        CREATE TABLE fruit (
            id INT IDENTITY(1,1) PRIMARY KEY,
            name NVARCHAR(100) NOT NULL,
            colour NVARCHAR(100) NOT NULL,
            price INT
        )`
	}
	panic("unknown db type")
}

// initializeTestData sets up initial test data in the database
func initializeTestData(db *sql.DB, dbType DBType) error {
	createTableSQL := getCreateTableSQL(dbType)
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// Insert test data - using parameterized queries for better compatibility
	var insertDataSQL string
	switch dbType {
	case Postgres:
		insertDataSQL = "INSERT INTO fruit (name, colour, price) VALUES ($1, $2, $3)"
	case MSSQL:
		insertDataSQL = "INSERT INTO fruit (name, colour, price) VALUES (@p1, @p2, @p3)"
	default:
		insertDataSQL = "INSERT INTO fruit (name, colour, price) VALUES (?, ?, ?)"
	}

	for _, f := range []struct {
		name   string
		colour string
		price  int
	}{
		{"apple", "green", 10},
		{"banana", "yellow", 20},
		{"cherry", "red", 11},
	} {
		if _, err := db.Exec(insertDataSQL, f.name, f.colour, f.price); err != nil {
			return fmt.Errorf("failed to insert test data: %v", err)
		}
	}
	if _, err := db.Exec(insertDataSQL, "orange", "orange", nil); err != nil {
		return fmt.Errorf("failed to insert test data: %v", err)
	}

	return nil
}

// setupDB initializes a test database of the specified type
func setupDB(t *testing.T, dbType DBType) (*TestConfig, func()) {
	t.Helper()
	ctx := context.Background()

	container, dbURL, err := setupTestContainer(ctx, dbType)
	if err != nil {
		t.Fatalf("failed to setup test container: %v", err)
	}

	typ := string(dbType)
	if dbType == Postgres {
		typ = "postgres"
	}
	db, err := sql.Open(typ, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	if dbType == SQLite {
		db.SetMaxOpenConns(1)
	}

	if err := initializeTestData(db, dbType); err != nil {
		t.Fatalf("failed to initialize test data: %v", err)
	}

	cleanup := func() {
		if container == nil {
			return
		}
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database connection: %v", err)
		}
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}

	return &TestConfig{
		db:     db,
		dbName: "testdb",
		dbType: dbType,
		dbURL:  dbURL,
	}, cleanup
}

type fruitRow struct {
	ID     int
	Name   string
	Colour string
	Price  sql.NullInt64
}

type fruitJSON struct {
	ID     int
	Name   string
	Colour string
	Price  *int
}

func toFruitRows(xs []fruitJSON) []fruitRow {
	rows := make([]fruitRow, len(xs))
	for i, x := range xs {
		rows[i] = fruitRow{
			ID:     x.ID,
			Name:   x.Name,
			Colour: x.Colour,
		}
		if x.Price != nil {
			rows[i].Price = sql.NullInt64{Int64: int64(*x.Price), Valid: true}
		}
	}
	return rows
}

// In these test, we test the compile API end-to-end. We start an instance of
// EOPA, load a policy, and then run a series of tests that compile a query and
// then execute it against a database. The query is a simple "include" query
// that filters rows from a table based on some conditions. The conditions are
// defined in the data policy.
func TestCompileHappyPathE2E(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	params := e2e.NewAPIServerTestParams()
	params.Addrs = &[]string{"0.0.0.0:0"}
	testRuntime, err := e2e.NewTestRuntime(params)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan bool)
	go func() {
		err := testRuntime.Runtime.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()
	t.Cleanup(cancel)

	if err := testRuntime.WaitForServer(); err != nil {
		t.Fatal(err)
	}
	opaURL := testRuntime.URL()
	t.Log(opaURL)

	dbTypes := []DBType{Postgres, MySQL, MSSQL, SQLite}

	unknowns := []string{"input.fruits"}
	var input any = map[string]any{"fav_colour": "yellow"}
	path := `filters%d/include`

	mapping := map[string]any{
		"fruits": map[string]any{
			"$self": "fruit",
		},
	}

	price := func(x int) sql.NullInt64 { return sql.NullInt64{Int64: int64(x), Valid: true} }
	apple := fruitRow{ID: 1, Name: "apple", Colour: "green", Price: price(10)}
	banana := fruitRow{ID: 2, Name: "banana", Colour: "yellow", Price: price(20)}
	cherry := fruitRow{ID: 3, Name: "cherry", Colour: "red", Price: price(11)}
	orange := fruitRow{ID: 4, Name: "orange", Colour: "orange"}

	tests := []struct {
		name    string
		policy  string
		expRows []fruitRow
		prisma  bool
		exclude []DBType
	}{
		{
			name:    "no conditions",
			policy:  `include if true`,
			expRows: []fruitRow{apple, banana, cherry, orange},
			prisma:  true,
		},
		{
			name:   "unconditional NO",
			policy: `include if false`,
			prisma: true,
		},
		{
			name:    "simple equality",
			policy:  `include if input.fruits.colour == input.fav_colour`,
			expRows: []fruitRow{banana},
			prisma:  true,
		},
		{
			name:    "comparison with two unknowns",
			policy:  `include if input.fruits.price >= input.fruits.id`,
			expRows: []fruitRow{apple, banana, cherry},
		},
		{
			name:    "simple comparison",
			policy:  `include if input.fruits.price < 11`,
			expRows: []fruitRow{apple},
			prisma:  true,
		},
		{
			name:    "equal null",
			policy:  `include if input.fruits.price == null`,
			expRows: []fruitRow{orange},
			prisma:  true,
		},
		{
			name:    "not equal null",
			policy:  `include if input.fruits.price != null`,
			expRows: []fruitRow{apple, banana, cherry},
			prisma:  true,
		},
		{
			name:    "simple startswith",
			policy:  `include if startswith(input.fruits.name, "app")`,
			expRows: []fruitRow{apple},
			prisma:  true,
			exclude: []DBType{SQLite},
		},
		{
			name:    "simple contains",
			policy:  `include if contains(input.fruits.name, "a")`,
			expRows: []fruitRow{apple, banana, orange},
			prisma:  true,
			exclude: []DBType{SQLite},
		},
		{
			name:    "startswith + escaping '_'",
			policy:  `include if startswith(input.fruits.name, "ap_")`, // if "_" wasn't escaped properly, it would match "apple"
			expRows: nil,
			exclude: []DBType{SQLite},
		},
		{
			name:    "startswith + escaping '%'",
			policy:  `include if startswith(input.fruits.name, "%ppl")`, // if "%" wasn't escaped properly, it would match "apple"
			expRows: nil,
			exclude: []DBType{SQLite},
		},
		{
			name:    "simple endswith",
			policy:  `include if endswith(input.fruits.name, "le")`,
			expRows: []fruitRow{apple},
			prisma:  true,
			exclude: []DBType{SQLite},
		},
		{
			name:    "internal.member_2",
			policy:  `include if input.fruits.name in {"apple", "cherry", "pineapple"}`,
			expRows: []fruitRow{apple, cherry},
			prisma:  true,
		},
		{
			name: "conjunct query, inequality",
			policy: `include if {
				input.fruits.name != "apple"
				input.fruits.name != "banana"
				}`,
			expRows: []fruitRow{cherry, orange},
			prisma:  true,
		},
		{
			name: "disjunct query, equality",
			policy: `include if input.fruits.name == "apple"
				include if input.fruits.name == "banana"`,
			expRows: []fruitRow{apple, banana},
			prisma:  true,
		},
		{
			name:    "not+internal.member_2",
			policy:  `include if not input.fruits.name in {"apple", "cherry", "pineapple", "orange"}`,
			expRows: []fruitRow{banana},
			prisma:  true,
		},
		{
			name:    "not+lt",
			policy:  `include if not input.fruits.price < 12`,
			expRows: []fruitRow{banana},
			prisma:  true,
		},
	}

	for _, dbType := range dbTypes {
		t.Run(string(dbType), func(t *testing.T) {
			t.Parallel()
			config, cleanup := setupDB(t, dbType)
			t.Cleanup(cleanup)

			for i, tt := range tests {
				// first, we override the policy with the current test case
				policy := fmt.Sprintf("package filters%d\n%s", i, tt.policy)
				req, err := http.NewRequest("PUT", fmt.Sprintf("%s/v1/policies/policy%d.rego", opaURL, i), strings.NewReader(policy))
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}
				if _, err := http.DefaultClient.Do(req); err != nil {
					t.Fatalf("post policy: %v", err)
				}

				path := fmt.Sprintf(path, i)

				if tt.prisma && dbType == Postgres {
					t.Run("prisma/"+tt.name, func(t *testing.T) { // also run via our prisma contraption
						t.Parallel()

						// second, query the compile API
						payload := map[string]any{
							"input":    input,
							"unknowns": unknowns,
							"options": map[string]any{
								"targetSQLTableMappings": map[string]any{
									"ucast": mapping,
								},
							},
						}
						// get the UCAST IR, process with @styra/ucast-prisma, and run a findMany query
						// with that against the DB, return rows
						rowsData := getUCASTAndRunPrisma(t, path, payload, config, opaURL)

						if diff := cmp.Diff(tt.expRows, rowsData); diff != "" {
							t.Errorf("unexpected result (-want +got):\n%s", diff)
						}
					})
				}
				t.Run("db/"+tt.name, func(t *testing.T) {
					t.Parallel()
					if slices.Contains(tt.exclude, dbType) {
						t.Skip("skipped via exclude")
					}

					// second, query the compile API
					mappings := make(map[string]any, 1)
					mappings[string(config.dbType)] = mapping
					payload := map[string]any{
						"input":    input,
						"unknowns": unknowns,
						"options": map[string]any{
							"targetSQLTableMappings": mappings,
						},
					}

					// get the SQL where clauses, and run the concatenated query against db,
					// return rows
					rowsData := getSQLAndRunQuery(t, path, payload, config, opaURL)

					// finally, compare with expected!
					if diff := cmp.Diff(tt.expRows, rowsData); diff != "" {
						t.Errorf("unexpected result (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

// This test runs three Compile API queries and asserts that the handler
// execution added something to the exposed prometheus metrics at /v1/metrics.
// Also, it checks that the cache hit/miss metrics have been exposed accordingly.
func TestPrometheusMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	params := e2e.NewAPIServerTestParams()
	params.Addrs = &[]string{"0.0.0.0:0"}
	testRuntime, err := e2e.NewTestRuntime(params)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan bool)
	go func() {
		err := testRuntime.Runtime.Serve(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		done <- true
	}()
	t.Cleanup(cancel)
	if err := testRuntime.WaitForServer(); err != nil {
		t.Fatal(err)
	}

	opaURL := testRuntime.URL()

	input := map[string]any{"fav_colour": "yellow"}
	path := `filters/include`
	policy := `package filters
# METADATA
# scope: document
# custom:
#   unknowns: [input.fruits]
include if input.fruits.name == "banana"
`

	{ // exercise the Compile API
		req, err := http.NewRequest("PUT", opaURL+"/v1/policies/policy.rego", strings.NewReader(policy))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		if _, err := http.DefaultClient.Do(req); err != nil {
			t.Fatalf("post policy: %v", err)
		}

		// query the compile API
		payload := map[string]any{
			"input": input,
		}

		queryBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}

		for range 3 {
			// POST to Compile API
			req, err = http.NewRequest("POST",
				opaURL+"/v1/compile/"+path,
				strings.NewReader(string(queryBytes)))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/vnd.opa.sql.postgresql+json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to execute request: %v", err)
			}
			defer resp.Body.Close()
			var respPayload struct {
				Result struct {
					Query any `json:"query"`
				} `json:"result"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if exp, act := "WHERE fruits.name = E'banana'", respPayload.Result.Query; exp != act {
				t.Errorf("response: expected %v, got %v (response: %v)", exp, act, respPayload)
			}
		}
	}

	{ // check the /v1/metrics endpoint for the handler invokation lines
		needle := `http_request_duration_seconds_bucket{code="200",handler="v1/compile",method="post",`
		act := findMetrics(t, opaURL, needle)
		// NOTE(sr): The metric is a histogram, and we did multiple requests; so we only check
		// for ANY lines being present, not their actual count.
		if len(act) < 1 {
			t.Errorf("expected v1/compile lines in metrics, got %v", act)
		}
	}
}

func findMetrics(t *testing.T, eopaURL string, needles ...string) []string {
	t.Helper()
	founds := []string{}
	resp, err := http.Get(eopaURL + "/metrics")
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	// search for line containing v1/compile
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		for i := range needles {
			if strings.Contains(scanner.Text(), needles[i]) {
				founds = append(founds, scanner.Text())
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan response: %v", err)
	}
	return founds
}

func getSQLAndRunQuery(t *testing.T, path string, payload map[string]any, config *TestConfig, opaURL string) []fruitRow {
	t.Helper()
	queryBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST",
		opaURL+"/v1/compile/"+path,
		strings.NewReader(string(queryBytes)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", fmt.Sprintf("application/vnd.opa.sql.%s+json", config.dbType))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if status := resp.StatusCode; status != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, status)
	}

	var respPayload struct {
		Result struct {
			Query any `json:"query"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	t.Log(respPayload)
	var rowsData []fruitRow
	var whereClauses string
	switch w := respPayload.Result.Query.(type) {
	case nil: // unconditional NO
		return rowsData // empty
	case string:
		whereClauses = w
	}

	// finally, query the database with the resulting WHERE clauses
	stmt := "SELECT * FROM fruit " + whereClauses
	rows, err := config.db.Query(stmt)
	if err != nil {
		t.Fatalf("%s: error: %v", stmt, err)
	}
	// collect rows into rowsData
	for rows.Next() {
		var fruit fruitRow
		// scan row into fruit, ignoring created_at
		if err := rows.Scan(&fruit.ID, &fruit.Name, &fruit.Colour, &fruit.Price); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		rowsData = append(rowsData, fruit)
	}
	return rowsData
}

func getUCASTAndRunPrisma(t *testing.T, path string, payload map[string]any, config *TestConfig, opaURL string) []fruitRow {
	t.Helper()
	queryBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Query EOPA for UCAST IR
	req, err := http.NewRequest("POST",
		opaURL+"/v1/compile/"+path,
		strings.NewReader(string(queryBytes)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.opa.ucast.prisma+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if status := resp.StatusCode; status != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, status)
	}

	var respPayload struct {
		Result struct {
			Query map[string]any `json:"query"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	t.Log("ucast IR:", respPayload.Result.Query)

	// Execute prisma script with UCAST IR as input
	cmd := exec.Command("node", "index.js")
	cmd.Dir = "./prisma"
	cmd.Env = append(cmd.Env, "DATABASE_URL="+config.dbURL)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start prisma script: %v", err)
	}

	// Write UCAST IR to stdin
	if err := json.NewEncoder(stdin).Encode(respPayload.Result.Query); err != nil {
		t.Fatalf("failed to write to stdin: %v", err)
	}
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		t.Fatalf("prisma script failed: %v\nstderr: %s", err, stderr.String())
	}
	t.Log("prisma query:", stderr.String())

	// Parse the output into []fruitRow
	if stdout.Len() == 0 {
		return nil
	}
	var rowsData []fruitJSON
	if err := json.NewDecoder(&stdout).Decode(&rowsData); err != nil {
		t.Fatalf("failed to decode prisma output: %v\noutput was: %s", err, stdout.String())
	}

	return toFruitRows(rowsData)
}
