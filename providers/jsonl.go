package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// LoadJSONL reads newline-delimited JSON objects from r. Every non-blank,
// non-`#` line must be a JSON object whose top-level keys become row columns.
// firstKeys carries the keys of the first object in their JSON-document order
// so the caller can use them as the default column projection (the table
// stays in the order the user piped in instead of being re-sorted
// alphabetically). UseNumber preserves int/float distinctions, matching
// loadJSON.
func LoadJSONL(r io.Reader) (rows []map[string]any, firstKeys []string, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if firstKeys == nil {
			keys, kerr := jsonObjectKeysInOrder(line)
			if kerr != nil {
				return nil, nil, fmt.Errorf("decode JSONL line %d: %w", lineNum, kerr)
			}
			firstKeys = keys
		}
		var row map[string]any
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		if derr := dec.Decode(&row); derr != nil {
			return nil, nil, fmt.Errorf("decode JSONL line %d: %w", lineNum, derr)
		}
		rows = append(rows, row)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, nil, scanErr
	}
	return rows, firstKeys, nil
}

// jsonObjectKeysInOrder returns the top-level keys of a JSON object in the
// order they appear in the source bytes. encoding/json's map decoder loses
// order, so we walk tokens manually for the first row only.
func jsonObjectKeysInOrder(line string) ([]string, error) {
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected JSON object, got %v", tok)
	}
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %v", keyTok)
		}
		keys = append(keys, key)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
	}
	return keys, nil
}
