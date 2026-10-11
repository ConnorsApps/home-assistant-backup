package main

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	swagger "github.com/swaggest/jsonschema-go"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

// The patterns must agree with the parsers the app and the API server use: a
// false reject would block a valid install.
func TestQuantityPattern(t *testing.T) {
	re := regexp.MustCompile(quantityPattern)
	for _, s := range []string{"64Mi", "500m", "1.5", "5Gi", "1e3", "1E3", "1e-3", "0", "+1", "-1", ".5", "1k", "1K", "1Ki", "1ki", "1M", "1Mi", "big", "", "1 Gi", "1.Gi"} {
		_, err := resource.ParseQuantity(s)
		if got := re.MatchString(s); got != (err == nil) {
			t.Errorf("%q: pattern match = %v, ParseQuantity ok = %v", s, got, err == nil)
		}
	}
}

// HASS_TIMEOUT is passed to time.ParseDuration, and the app rejects "".
func TestTimeoutPattern(t *testing.T) {
	re := regexp.MustCompile(timeoutPattern)
	for _, s := range []string{"", "0", "+0", "30s", "1h30m", "1.5h", "500ms", "5.s", ".5s", "1µs", "1μs", "1us", "1ns", "-5s", "+5s", "5", "10", "s", "1d", "1 s", "0s", "00", "10m", "2160h"} {
		_, err := time.ParseDuration(s)
		if got := re.MatchString(s); got != (err == nil) {
			t.Errorf("%q: pattern match = %v, ParseDuration ok = %v", s, got, err == nil)
		}
	}
}

// RETENTION_KEEP_LAST is read with strconv.Atoi and must not be negative.
func TestKeepLastPattern(t *testing.T) {
	re := regexp.MustCompile(keepLastPattern)
	for _, s := range []string{"0", "30", "+5", "-0", "05", "-1", "-5", "1.5", "", "abc", " 5", "5 ", "1e3", "0x10", "1_000"} {
		n, err := strconv.Atoi(s)
		if got, want := re.MatchString(s), err == nil && n >= 0; got != want {
			t.Errorf("%q: pattern match = %v, Atoi ok and >= 0 = %v", s, got, want)
		}
	}
}

// HASS_INSECURE and HASS_DELETE_AFTER_TRANSFER are read with strconv.ParseBool.
func TestBoolishEnum(t *testing.T) {
	var s swagger.Schema
	if err := boolish("").PrepareJSONSchema(&s); err != nil {
		t.Fatal(err)
	}
	enum := map[string]bool{}
	for _, v := range s.Enum {
		if str, ok := v.(string); ok {
			enum[str] = true
		}
	}
	for _, in := range []string{
		"1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False",
		"yes", "no", "y", "n", "on", "off", "", "2", "tru", "tRUE", "TrUe", " true",
	} {
		_, err := strconv.ParseBool(in)
		if enum[in] != (err == nil) {
			t.Errorf("%q: in enum = %v, ParseBool ok = %v", in, enum[in], err == nil)
		}
	}
}

// The app lowercases LOG_LEVEL and LOG_FORMAT before comparing (pkg/config).
func TestLogPatterns(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pattern string
		valid   []string
	}{
		{"level", anyCase(logLevels...), logLevels},
		{"format", anyCase(logFormats...), logFormats},
	} {
		re := regexp.MustCompile(tc.pattern)
		for _, in := range []string{
			"debug", "DEBUG", "Debug", "dEbUg", "info", "INFO", "warn", "Warn", "WARNING", "warning", "error", "ERROR",
			"pretty", "Pretty", "TEXT", "text", "json", "JSON", "Json",
			"", "loud", "trace", "fatal", "xml", " debug", "debug ", "debu", "debugg",
		} {
			want := slices.Contains(tc.valid, strings.ToLower(in))
			if got := re.MatchString(in); got != want {
				t.Errorf("%s %q: pattern match = %v, app accepts = %v", tc.name, in, got, want)
			}
		}
	}
}

func TestCronPattern(t *testing.T) {
	re := regexp.MustCompile(cronPattern)
	for s, want := range map[string]bool{
		"0 3 * * *": true, "*/5 * * * *": true, "30 3 * * MON-FRI": true, "@daily": true, "@every 1h": true,
		"CRON_TZ=UTC 0 3 * * *": true, "0 3 * *": false, "0 3 * * * *": false, "nope": false, "": false, "@never": false,
	} {
		if got := re.MatchString(s); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

// The app opens the URL with gocloud.dev, and only s3, gs and file are built in.
func TestStorageURLPattern(t *testing.T) {
	re := regexp.MustCompile(storageURLPattern)
	for s, want := range map[string]bool{
		"s3://my-bucket": true, "s3://my-bucket?use_path_style=true": true, "gs://my-bucket": true,
		"file:///backups": true, "file://./backups": true,
		"": false, "s3://": false, "my-bucket": false, "https://example.com/bucket": false, "azblob://c": false,
	} {
		if got := re.MatchString(s); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

func TestEnvNamePattern(t *testing.T) {
	re := regexp.MustCompile(envNamePattern)
	for s, want := range map[string]bool{
		"HASS_URL": true, "aws.region": true, "a-b": true, "_X": true, "X1": true,
		"": false, "1X": false, "A B": false, "A=B": false,
	} {
		if got := re.MatchString(s); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}

// compile builds the schema the way main does.
func compile(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := build()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("values.schema.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("values.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validateYAML(schema *jsonschema.Schema, doc string) error {
	j, err := yaml.YAMLToJSON([]byte(doc))
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(j, &v); err != nil {
		return err
	}
	return schema.Validate(v)
}

// The schema must reject what the templates or the app would choke on, and
// accept what they tolerate.
func TestValidation(t *testing.T) {
	schema := compile(t)
	for name, tc := range map[string]struct {
		doc string
		ok  bool
	}{
		"unknown top-level key":           {"bogus: 1", false},
		"global added to every subchart":  {"global: {}", true},
		"global from a parent chart":      {"global: {imageRegistry: example.com}", true},
		"enabled from a parent condition": {"enabled: true", true},
		"enabled must be a boolean":       {"enabled: maybe", false},
		"typo in nested key":              {"image: {pullPolcy: Always}", false},
		"typo in prometheusRule":          {"prometheusRule: {enable: true}", false},
		"overrides the helpers read":      {"nameOverride: hb\nfullnameOverride: home-backup", true},
		"bad name override":               {"fullnameOverride: Bad_Name", false},
		"bad pull policy":                 {"image: {pullPolicy: Sometimes}", false},
		"empty pull policy":               {"image: {pullPolicy: ''}", true},
		"numeric image tag from CI":       {"image: {tag: 1234}", true},
		"float image tag":                 {"image: {tag: 1.10}", false},
		"empty repository":                {"image: {repository: ''}", false},
		"empty tag derives from backend":  {"image: {tag: ''}", true},
		"cron":                            {"schedule: '*/5 * * * *'", true},
		"bad cron":                        {"schedule: nope", false},
		"negative history":                {"failedJobsHistoryLimit: -1", false},
		"history as string":               {"successfulJobsHistoryLimit: '3'", false},
		"s3 backend":                      {"env: {STORAGE_URL: 's3://my-bucket?use_path_style=true', AWS_ENDPOINT_URL: 'https://ceph.example.com'}", true},
		"gcs backend":                     {"env: {STORAGE_URL: 'gs://b'}\nsecret: {GOOGLE_APPLICATION_CREDENTIALS: /creds/sa.json}", true},
		"file backend":                    {"env: {STORAGE_URL: 'file:///backups'}", true},
		"unsupported backend":             {"env: {STORAGE_URL: 'azblob://b'}", false},
		"empty storage url":               {"env: {STORAGE_URL: ''}", false},
		"bad log level":                   {"env: {LOG_LEVEL: loud}", false},
		"log level":                       {"env: {LOG_LEVEL: debug}", true},
		"log level in capitals":           {"env: {LOG_LEVEL: DEBUG}", true},
		"log format in any case":          {"env: {LOG_FORMAT: Json}", true},
		"bad log format":                  {"env: {LOG_FORMAT: xml}", false},
		"keep last as number":             {"env: {RETENTION_KEEP_LAST: 7}", true},
		"keep last as string":             {"env: {RETENTION_KEEP_LAST: '30'}", true},
		"keep last zero":                  {"env: {RETENTION_KEEP_LAST: 0}", true},
		"keep last negative":              {"env: {RETENTION_KEEP_LAST: -1}", false},
		"keep last word":                  {"env: {RETENTION_KEEP_LAST: all}", false},
		"timeout":                         {"env: {HASS_TIMEOUT: 30m}", true},
		"timeout zero":                    {"env: {HASS_TIMEOUT: 0}", true},
		"timeout without unit":            {"env: {HASS_TIMEOUT: 10}", false},
		"empty timeout":                   {"env: {HASS_TIMEOUT: ''}", false},
		"insecure bool":                   {"env: {HASS_INSECURE: true}", true},
		"insecure string":                 {"env: {HASS_INSECURE: 'false'}", true},
		"insecure junk":                   {"env: {HASS_INSECURE: 'yes-please'}", false},
		"delete after transfer 0":         {"env: {HASS_DELETE_AFTER_TRANSFER: 0}", true},
		"env passes other scalars":        {"env: {HASS_MODE: core, SOME_FLAG: true, SOME_NUM: 5}", true},
		"env rejects objects":             {"env: {EXTRA: {a: b}}", false},
		"env rejects bad names":           {"env: {'1BAD': x}", false},
		"secret strings":                  {"secret: {HASS_TOKEN: abc, EXTRA_VAR: x}", true},
		"secret number":                   {"secret: {AWS_ACCESS_KEY_ID: 12345}", false},
		"secret bad name":                 {"secret: {'bad name': x}", false},
		"empty secret":                    {"secret: {}", true},
		"resources":                       {"resources: {requests: {memory: 64Mi, cpu: 100m}, limits: {memory: 256Mi}}", true},
		"bad resources":                   {"resources: {limits: {memory: lots}}", false},
		"prometheus rule":                 {"prometheusRule: {enabled: true, namespace: monitoring, labels: {release: prom}}", true},
		"bad rule namespace":              {"prometheusRule: {namespace: Monitoring}", false},
		"numeric rule label":              {"prometheusRule: {labels: {n: 1}}", false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateYAML(schema, tc.doc); (err == nil) != tc.ok {
				t.Errorf("valid = %v, want %v: %v", err == nil, tc.ok, err)
			}
		})
	}
}

// The chart's own defaults must validate: the schema is closed, so a key
// missing from values.go would otherwise break installs.
func TestRepoValuesValidate(t *testing.T) {
	y, err := os.ReadFile("../../charts/hass-backup/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateYAML(compile(t), string(y)); err != nil {
		t.Error(err)
	}
}

// schemaURL is where editors fetch the schema: the generated file on main.
const schemaURL = "https://raw.githubusercontent.com/ConnorsApps/home-assistant-backup/main/charts/hass-backup/values.schema.json"

// values.yaml and the README values examples start with a yaml-language-server
// comment for this schema. A typo there silently turns editor validation off,
// and an example that doesn't validate teaches the wrong thing.
func TestSchemaComments(t *testing.T) {
	schema := compile(t)
	header := "# yaml-language-server: $schema=" + schemaURL + "\n"

	y, err := os.ReadFile("../../charts/hass-backup/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(y), header) {
		t.Errorf("values.yaml must start with %q", strings.TrimSpace(header))
	}

	fence := regexp.MustCompile("(?s)```yaml\n(.*?)```")
	for _, f := range []string{"../../README.md"} {
		md, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		examples := 0
		for _, m := range fence.FindAllStringSubmatch(string(md), -1) {
			if !strings.HasPrefix(m[1], "# yaml-language-server:") {
				continue
			}
			examples++
			if !strings.HasPrefix(m[1], header) {
				t.Errorf("%s: example has the wrong schema comment: %q", f, strings.SplitN(m[1], "\n", 2)[0])
			}
			if err := validateYAML(schema, m[1]); err != nil {
				t.Errorf("%s: example does not validate: %v", f, err)
			}
		}
		if examples == 0 {
			t.Errorf("%s: no yaml example with a schema comment", f)
		}
	}
}
