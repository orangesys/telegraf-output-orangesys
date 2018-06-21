package orangesys_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/internal/tls"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/plugins/outputs/orangesys"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	URLF            func() string
	DatabaseF       func() string
	WriteF          func(context.Context, []telegraf.Metric) error
	CreateDatabaseF func(ctx context.Context) error
}

func (c *MockClient) URL() string {
	return c.URLF()
}

func (c *MockClient) Database() string {
	return c.DatabaseF()
}

func (c *MockClient) Write(ctx context.Context, metrics []telegraf.Metric) error {
	return c.WriteF(ctx, metrics)
}

func (c *MockClient) CreateDatabase(ctx context.Context) error {
	return c.CreateDatabaseF(ctx)
}

func TestDefaultURL(t *testing.T) {
	var actual *orangesys.HTTPConfig
	output := orangesys.Orangesys{
		CreateHTTPClientF: func(config *orangesys.HTTPConfig) (orangesys.Client, error) {
			actual = config
			return &MockClient{
				CreateDatabaseF: func(ctx context.Context) error {
					return nil
				},
			}, nil
		},
	}
	err := output.Connect()
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8086", actual.URL.String())
}

func TestConnectHTTPConfig(t *testing.T) {
	var actual *orangesys.HTTPConfig

	output := orangesys.Orangesys{
		URLs:             []string{"http://localhost:8086"},
		Database:         "telegraf",
		RetentionPolicy:  "default",
		WriteConsistency: "any",
		Timeout:          internal.Duration{Duration: 5 * time.Second},
		Username:         "guy",
		Password:         "smiley",
		UserAgent:        "telegraf",
		HTTPProxy:        "http://localhost:8086",
		HTTPHeaders: map[string]string{
			"x": "y",
		},
		JwtToken:        "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		ContentEncoding: "gzip",
		ClientConfig: tls.ClientConfig{
			InsecureSkipVerify: true,
		},

		CreateHTTPClientF: func(config *orangesys.HTTPConfig) (orangesys.Client, error) {
			actual = config
			return &MockClient{
				CreateDatabaseF: func(ctx context.Context) error {
					return nil
				},
			}, nil
		},
	}
	err := output.Connect()
	require.NoError(t, err)

	require.Equal(t, output.URLs[0], actual.URL.String())
	require.Equal(t, output.UserAgent, actual.UserAgent)
	require.Equal(t, output.Timeout.Duration, actual.Timeout)
	require.Equal(t, output.Username, actual.Username)
	require.Equal(t, output.Password, actual.Password)
	require.Equal(t, output.HTTPProxy, actual.Proxy.String())
	require.Equal(t, output.HTTPHeaders, actual.Headers)
	require.Equal(t, output.ContentEncoding, actual.ContentEncoding)
	require.Equal(t, output.Database, actual.Database)
	require.Equal(t, output.RetentionPolicy, actual.RetentionPolicy)
	require.Equal(t, output.WriteConsistency, actual.Consistency)
	require.NotNil(t, actual.TLSConfig)
	require.NotNil(t, actual.Serializer)

	require.Equal(t, output.Database, actual.Database)
}

func TestWriteRecreateDatabaseIfDatabaseNotFound(t *testing.T) {
	output := orangesys.Orangesys{
		URLs: []string{"http://localhost:8086"},

		CreateHTTPClientF: func(config *orangesys.HTTPConfig) (orangesys.Client, error) {
			return &MockClient{
				CreateDatabaseF: func(ctx context.Context) error {
					return nil
				},
				WriteF: func(ctx context.Context, metrics []telegraf.Metric) error {
					return &orangesys.APIError{
						StatusCode:  http.StatusNotFound,
						Title:       "404 Not Found",
						Description: `database not found "telegraf"`,
						Type:        orangesys.DatabaseNotFound,
					}
				},
				URLF: func() string {
					return "http://localhost:8086"

				},
			}, nil
		},
	}

	err := output.Connect()
	require.NoError(t, err)

	m, err := metric.New(
		"cpu",
		map[string]string{},
		map[string]interface{}{
			"value": 42.0,
		},
		time.Unix(0, 0),
	)
	require.NoError(t, err)
	metrics := []telegraf.Metric{m}

	err = output.Write(metrics)
	// We only have one URL, so we expect an error
	require.Error(t, err)
}
