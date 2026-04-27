package providers

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestLoadExternalSuccess(t *testing.T) {
	rows, err := loadExternal("testdata/echo.sh", Context{})
	if err != nil {
		t.Fatalf("loadExternal: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	if got := rows[0]["key"]; got != "alpha" {
		t.Errorf("rows[0].key = %v, want alpha", got)
	}
	// json.Number preserves the on-wire form so downstream comparisons keep
	// integer vs float distinctions where it matters.
	if got, ok := rows[0]["n"].(json.Number); !ok || got.String() != "1" {
		t.Errorf("rows[0].n = %T(%v), want json.Number(1)", rows[0]["n"], rows[0]["n"])
	}
	if got := rows[0]["ok"]; got != true {
		t.Errorf("rows[0].ok = %v, want true", got)
	}
	if got := rows[1]["key"]; got != "beta" {
		t.Errorf("rows[1].key = %v, want beta", got)
	}
	if got, ok := rows[1]["n"].(json.Number); !ok || got.String() != "2.5" {
		t.Errorf("rows[1].n = %T(%v), want json.Number(2.5)", rows[1]["n"], rows[1]["n"])
	}
}

func TestLoadExternalNonZeroExit(t *testing.T) {
	rows, err := loadExternal("testdata/error.sh", Context{})
	if err == nil {
		t.Fatalf("expected error, got rows %+v", rows)
	}
	if !strings.Contains(err.Error(), "error.sh") {
		t.Errorf("error should mention script path, got %q", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows on error, got %+v", rows)
	}
}

func TestLoadExternalMalformedLineSkipped(t *testing.T) {
	rows, err := loadExternal("testdata/malformed.sh", Context{})
	if err != nil {
		t.Fatalf("loadExternal: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (malformed line should be skipped): %+v", len(rows), rows)
	}
	if rows[0]["key"] != "first" || rows[1]["key"] != "second" {
		t.Errorf("got keys %v, %v; want first, second", rows[0]["key"], rows[1]["key"])
	}
}

func TestLoadExternalMissingExecutable(t *testing.T) {
	_, err := loadExternal("testdata/does-not-exist.sh", Context{})
	if err == nil {
		t.Fatal("expected error for missing executable")
	}
}

func TestLoadExternalPayloadDelivered(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	ctx := Context{
		Source: "regions.yaml",
		Prefix: "*.servers.*",
		Where:  "cpu > 8",
	}
	rows, err := loadExternal("testdata/echo_stdin.sh", ctx)
	if err != nil {
		t.Fatalf("loadExternal: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if got, ok := r["version"].(json.Number); !ok || got.String() != "1" {
		t.Errorf("version: got %T(%v), want json.Number(1)", r["version"], r["version"])
	}
	if r["source"] != "regions.yaml" {
		t.Errorf("source: got %v, want regions.yaml", r["source"])
	}
	if r["prefix"] != "*.servers.*" {
		t.Errorf("prefix: got %v, want *.servers.*", r["prefix"])
	}
	if r["where"] != "cpu > 8" {
		t.Errorf("where: got %v, want cpu > 8", r["where"])
	}
}

func TestReadExternalRowsSkipsBlankAndComment(t *testing.T) {
	in := strings.NewReader(`
# leading comment
{"type": "row", "value": {"a": 1}}


# another comment
{"type": "row", "value": {"b": 2}}
`)
	rows, err := readExternalRows(in, "")
	if err != nil {
		t.Fatalf("readExternalRows: %v", err)
	}
	want := []map[string]any{
		{"a": json.Number("1")},
		{"b": json.Number("2")},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("got %+v, want %+v", rows, want)
	}
}

func TestReadExternalRowsTreeExpands(t *testing.T) {
	in := strings.NewReader(`
{"type": "tree", "value": {"region-a": {"servers": {"web1": {"cpu": 8}, "db1": {"cpu": 32}}}}}
{"type": "tree", "value": {"region-b": {"servers": {"web1": {"cpu": 4}}}}}
`)
	rows, err := readExternalRows(in, "*.servers.*")
	if err != nil {
		t.Fatalf("readExternalRows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(rows), rows)
	}
	keys := make(map[string]json.Number)
	for _, r := range rows {
		k, _ := r["key"].(string)
		cpu, _ := r["cpu"].(json.Number)
		keys[k] = cpu
	}
	want := map[string]json.Number{
		"region-a.servers.web1": json.Number("8"),
		"region-a.servers.db1":  json.Number("32"),
		"region-b.servers.web1": json.Number("4"),
	}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("got %+v, want %+v", keys, want)
	}
}

func TestReadExternalRowsRejectsBareRow(t *testing.T) {
	in := strings.NewReader(`{"key": "alice"}` + "\n")
	rows, err := readExternalRows(in, "")
	if err != nil {
		t.Fatalf("readExternalRows: %v", err)
	}
	if rows != nil {
		t.Errorf("got %+v, want nil (bare object without envelope must be skipped)", rows)
	}
}

func TestReadExternalRowsMixedRowAndTree(t *testing.T) {
	in := strings.NewReader(`
{"type": "row", "value": {"key": "passthrough", "n": 1}}
{"type": "tree", "value": {"alice": {"age": 30}}}
`)
	rows, err := readExternalRows(in, "*")
	if err != nil {
		t.Fatalf("readExternalRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	want := []map[string]any{
		{"key": "passthrough", "n": json.Number("1")},
		{"key": "alice", "age": json.Number("30")},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("got %+v, want %+v", rows, want)
	}
}
