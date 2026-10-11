package main

import (
	"strings"

	jsonschema "github.com/swaggest/jsonschema-go"
	corev1 "k8s.io/api/core/v1"
)

// These types mirror charts/hass-backup/values.yaml by hand, plus the keys the
// templates read that it leaves out (nameOverride, fullnameOverride). The
// schema rejects unknown keys on the chart's own objects, so a key added to
// values.yaml without a field here fails `helm lint`. env and secret are
// open: see ownStruct (main.go). Their documented variables mirror
// pkg/config/config.go and CONFIGURATION.md in the app.

// pullPolicy: "" renders as null, which leaves the field to the cluster default.
type pullPolicy string

func (pullPolicy) Enum() []any {
	return []any{"", string(corev1.PullAlways), string(corev1.PullIfNotPresent), string(corev1.PullNever)}
}

// imageTag is a string, but a CI-supplied `--set image.tag=1234` parses as an
// integer and the helper prints it as is, so both must keep working. A float
// stays rejected: 1.10 would render as 1.1.
type imageTag string

func (imageTag) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.Type = &jsonschema.Type{SliceOfSimpleTypeValues: []jsonschema.SimpleType{jsonschema.String, jsonschema.Integer}}
	return nil
}

// cronSchedule is what a Kubernetes CronJob accepts: five fields, an @macro
// or @every, each optionally prefixed with a timezone.
type cronSchedule string

const cronPattern = `^((CRON_TZ|TZ)=\S+\s+)?(@(yearly|annually|monthly|weekly|daily|midnight|hourly)|@every\s+\S+|\S+(\s+\S+){4})$`

func (cronSchedule) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithPattern(cronPattern)
	return nil
}

// timeout is a time.ParseDuration string, which the app requires. YAML lets an
// unquoted 0 through as an integer, but no other integer parses: 10 has no unit.
type timeout string

const timeoutPattern = `^([-+]?(([0-9]+(\.[0-9]*)?|\.[0-9]+)(ns|us|µs|μs|ms|s|m|h))+|[-+]?0)$`

func (timeout) PrepareJSONSchema(s *jsonschema.Schema) error {
	str := (&jsonschema.Schema{}).WithType(jsonschema.String.Type()).WithPattern(timeoutPattern)
	zero := (&jsonschema.Schema{}).WithConst(0)
	s.Type = nil // the Go type is a string, which would rule out the 0 branch
	s.WithAnyOf(str.ToSchemaOrBool(), zero.ToSchemaOrBool())
	return nil
}

// boolish is what strconv.ParseBool accepts. The template quotes the value,
// so a YAML boolean or 0/1 works as well as the strings.
type boolish string

func (boolish) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.Type = &jsonschema.Type{SliceOfSimpleTypeValues: []jsonschema.SimpleType{jsonschema.Boolean, jsonschema.Integer, jsonschema.String}}
	s.WithEnum(true, false, 0, 1, "1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False")
	return nil
}

// keepLast is a non-negative integer for strconv.Atoi, as a number or a string.
type keepLast string

const keepLastPattern = `^(\+?[0-9]+|-0+)$`

func (keepLast) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.Type = &jsonschema.Type{SliceOfSimpleTypeValues: []jsonschema.SimpleType{jsonschema.Integer, jsonschema.String}}
	s.WithMinimum(0).WithPattern(keepLastPattern)
	return nil
}

// storageURL is a gocloud.dev bucket URL for a backend the app is built with.
type storageURL string

const storageURLPattern = `^(s3|gs|file)://.+$`

func (storageURL) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithPattern(storageURLPattern)
	return nil
}

// anyCase builds a pattern for words the app lowercases before comparing: JSON
// Schema patterns have no case-insensitive flag. A plain enum would reject
// LOG_LEVEL=DEBUG, which works.
func anyCase(words ...string) string {
	alts := make([]string, len(words))
	for i, w := range words {
		var b strings.Builder
		for _, r := range w {
			b.WriteString("[" + strings.ToUpper(string(r)) + string(r) + "]")
		}
		alts[i] = b.String()
	}
	return "^(" + strings.Join(alts, "|") + ")$"
}

var (
	logLevels  = []string{"debug", "info", "warn", "warning", "error"}
	logFormats = []string{"pretty", "text", "json"}
)

type logLevel string

func (logLevel) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithPattern(anyCase(logLevels...)).WithExamples("debug", "info", "warn", "error")
	return nil
}

type logFormat string

func (logFormat) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithPattern(anyCase(logFormats...)).WithExamples("pretty", "text", "json")
	return nil
}

// envNamePattern is what Kubernetes accepts as an environment variable name.
const envNamePattern = `^[-._a-zA-Z][-._a-zA-Z0-9]*$`

type imageValues struct {
	Repository string     `json:"repository" minLength:"1" description:"Image repository"`
	Tag        imageTag   `json:"tag" description:"Image tag. Empty: <appVersion>-<backend>, where the backend (s3, gcs or file) follows the scheme of env.STORAGE_URL."`
	PullPolicy pullPolicy `json:"pullPolicy" description:"Image pull policy. Empty: the cluster default."`
}

type envValues struct {
	HassURL                 string     `json:"HASS_URL" description:"Home Assistant URL, e.g. https://homeassistant.example.com. Required."`
	HassTimeout             timeout    `json:"HASS_TIMEOUT" description:"HTTP timeout for backup operations, e.g. 10m, 1h or 30s. App default: 10m."`
	HassInsecure            boolish    `json:"HASS_INSECURE" description:"Skip TLS verification, for self-signed certificates. App default: false."`
	HassDeleteAfterTransfer boolish    `json:"HASS_DELETE_AFTER_TRANSFER" description:"Delete the backup from Home Assistant once it is in storage. App default: true."`
	StorageURL              storageURL `json:"STORAGE_URL" description:"Where backups go: s3://bucket (S3-compatible), gs://bucket (Google Cloud Storage) or file:///path. The image variant (-s3, -gcs, -file) follows the scheme unless image.tag is set. Append ?use_path_style=true for S3-compatible services without virtual-hosted-style addressing (Ceph RGW, MinIO)."`
	StoragePrefix           string     `json:"STORAGE_PREFIX" description:"Object key prefix for backup files"`
	AWSEndpointURL          string     `json:"AWS_ENDPOINT_URL" description:"Endpoint of an S3-compatible service such as Ceph RGW or MinIO. Empty: AWS S3."`
	AWSRegion               string     `json:"AWS_REGION" description:"S3 region"`
	RetentionKeepLast       keepLast   `json:"RETENTION_KEEP_LAST" description:"Number of backups to keep; 0 keeps them all"`
	LogLevel                logLevel   `json:"LOG_LEVEL" description:"Log level: debug, info, warn or error, in any case"`
	LogFormat               logFormat  `json:"LOG_FORMAT" description:"Log format: pretty, text or json, in any case"`
}

type secretValues struct {
	HassToken                    string `json:"HASS_TOKEN" description:"Home Assistant long-lived access token. Required."`
	AWSAccessKeyID               string `json:"AWS_ACCESS_KEY_ID" description:"S3 access key ID"`
	AWSSecretAccessKey           string `json:"AWS_SECRET_ACCESS_KEY" description:"S3 secret access key"`
	GoogleApplicationCredentials string `json:"GOOGLE_APPLICATION_CREDENTIALS" description:"GCS only: path to the service account JSON, which you mount into the pod separately"`
}

type prometheusRuleValues struct {
	Enabled   bool              `json:"enabled" description:"Render a PrometheusRule that alerts on failed or missing backups. Needs the Prometheus Operator CRDs and kube-state-metrics."`
	Namespace string            `json:"namespace" pattern:"^([a-z0-9]([-a-z0-9]*[a-z0-9])?)?$" description:"Namespace of the PrometheusRule, which the Prometheus Operator must watch"`
	Labels    map[string]string `json:"labels" description:"Extra PrometheusRule labels, e.g. the one your Prometheus ruleSelector matches"`
}

type values struct {
	Global                     map[string]any              `json:"global" description:"Values shared with parent charts. Helm adds this key to every subchart, so it must be allowed; the chart reads none of it."`
	Enabled                    bool                        `json:"enabled" description:"Only for a parent chart that gates this chart with condition: <name>.enabled, which puts the key here. The chart itself does not read it."`
	NameOverride               string                      `json:"nameOverride" pattern:"^([a-z0-9]([-a-z0-9]*[a-z0-9])?)?$" description:"Override the chart name in resource names"`
	FullnameOverride           string                      `json:"fullnameOverride" pattern:"^([a-z0-9]([-a-z0-9]*[a-z0-9])?)?$" description:"Override the fully qualified resource name"`
	Image                      imageValues                 `json:"image"`
	Schedule                   cronSchedule                `json:"schedule" description:"CronJob schedule: five fields or an @macro such as @daily (default: 3 AM daily)"`
	SuccessfulJobsHistoryLimit int                         `json:"successfulJobsHistoryLimit" minimum:"0" description:"Finished Jobs to keep"`
	FailedJobsHistoryLimit     int                         `json:"failedJobsHistoryLimit" minimum:"0" description:"Failed Jobs to keep"`
	Env                        envValues                   `json:"env" description:"Non-sensitive environment variables for the backup container. Any other name is passed through too; see CONFIGURATION.md for what the app reads."`
	Secret                     secretValues                `json:"secret" description:"Sensitive environment variables, stored in a Secret. Values must be strings: use --set-string for numeric ones. Any other name is passed through too."`
	Resources                  corev1.ResourceRequirements `json:"resources" description:"Container resources"`
	PrometheusRule             prometheusRuleValues        `json:"prometheusRule" description:"Optional PrometheusRule alerting on failed or missing backups"`
}
