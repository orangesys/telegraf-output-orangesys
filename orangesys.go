package orangesys

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/internal/tls"
	"github.com/influxdata/telegraf/plugins/outputs"
	"github.com/influxdata/telegraf/plugins/serializers/influx"
)

var (
	defaultURL = "http://localhost:8086"

	// ErrMissingURL is error wiht url
	ErrMissingURL = errors.New("missing URL")
)

// Client is new orangeysys client
type Client interface {
	Write(context.Context, []telegraf.Metric) error
	CreateDatabase(ctx context.Context) error

	URL() string
	Database() string
}

// Orangesys struct is the primary data structure for the plugin
type Orangesys struct {
	// URL is only for backwards compatability
	URL                  string
	URLs                 []string `toml:"urls"`
	Username             string
	Password             string
	JwtToken             string `toml:"jwt_token"`
	Database             string
	UserAgent            string
	RetentionPolicy      string
	WriteConsistency     string
	Timeout              internal.Duration
	UDPPayload           int               `toml:"udp_payload"`
	HTTPProxy            string            `toml:"http_proxy"`
	HTTPHeaders          map[string]string `toml:"http_headers"`
	ContentEncoding      string            `toml:"content_encoding"`
	SkipDatabaseCreation bool              `toml:"skip_database_creation"`
	InfluxUintSupport    bool              `toml:"influx_uint_support"`
	tls.ClientConfig

	// Path to CA file
	SSLCA string `toml:"ssl_ca"`
	// Path to host cert file
	SSLCert string `toml:"ssl_cert"`
	// Path to cert key file
	SSLKey string `toml:"ssl_key"`
	// Use SSL but skip chain & host verification
	InsecureSkipVerify bool

	// Precision is only here for legacy support. It will be ignored.
	Precision string

	clients []Client

	CreateHTTPClientF func(config *HTTPConfig) (Client, error)

	serializer *influx.Serializer
}

var sampleConfig = `
  urls = ["https://<orangesys-url>"] # required
  database = "telegraf" # required
  jwt_token = "jwt_token" # required

  ## Compress each HTTP request payload using GZIP.
  # content_encoding = "gzip"
`

// Connect initiates the primary connection to the range of provided URLs
func (i *Orangesys) Connect() error {
	ctx := context.Background()

	urls := make([]string, 0, len(i.URLs))
	urls = append(urls, i.URLs...)

	if i.URL != "" {
		urls = append(urls, i.URL)
	}

	if len(urls) == 0 {
		urls = append(urls, defaultURL)
	}

	i.serializer = influx.NewSerializer()

	if i.InfluxUintSupport {
		i.serializer.SetFieldTypeSupport(influx.UintSupport)
	}

	for _, u := range urls {
		u, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("error parsing url [%s]: %v", u, err)
		}

		var proxy *url.URL
		if len(i.HTTPProxy) > 0 {
			proxy, err = url.Parse(i.HTTPProxy)
			if err != nil {
				return fmt.Errorf("error parsing proxy_url [%s]: %v", proxy, err)
			}
		}

		switch u.Scheme {
		case "http", "https", "unix":
			c, err := i.httpClient(ctx, u, proxy)
			if err != nil {
				return err
			}

			i.clients = append(i.clients, c)
		default:
			return fmt.Errorf("unsupport scheme [%s]: %q", u, u.Scheme)
		}
	}

	return nil
}

// Close will terminate the session to the backend, returning error if an issue arises
func (i *Orangesys) Close() error {
	return nil
}

// Description plugin output orangesys
func (i *Orangesys) Description() string {
	return "Configuration for sending metrics to Orangesys"
}

// SampleConfig returns the formatted sample configuration for the plugin
func (i *Orangesys) SampleConfig() string {
	return sampleConfig
}

// Write will choose a random server in the cluster to write to until a successful write
// occurs, logging each unsuccessful. If all servers fail, return error.
func (i *Orangesys) Write(metrics []telegraf.Metric) error {
	ctx := context.Background()

	var err error
	p := rand.Perm(len(i.clients))

	for _, n := range p {
		client := i.clients[n]
		err = client.Write(ctx, metrics)
		if err == nil {
			return nil
		}

		switch apiError := err.(type) {
		case *APIError:
			if !i.SkipDatabaseCreation {
				if apiError.Type == DatabaseNotFound {
					err := client.CreateDatabase(ctx)
					if err != nil {
						log.Printf("E! [outputs.orangesys] when write to [%s]: database %q not found and failed to recreate",
							client.URL(), client.Database())
					}
				}
			}
		}
		log.Printf("E! [outputs.influxdb]: when writing to [%s]: %v", client.URL(), err)
	}
	return errors.New("cloud not write any address")
}

func (i *Orangesys) httpClient(ctx context.Context, url *url.URL, proxy *url.URL) (Client, error) {
	tlsConfig, err := i.ClientConfig.TLSConfig()
	if err != nil {
		return nil, err
	}

	config := &HTTPConfig{
		URL:             url,
		Timeout:         i.Timeout.Duration,
		TLSConfig:       tlsConfig,
		UserAgent:       i.UserAgent,
		Username:        i.Username,
		Password:        i.Password,
		Proxy:           proxy,
		ContentEncoding: i.ContentEncoding,
		Headers:         i.HTTPHeaders,
		Database:        i.Database,
		JwtToken:        i.JwtToken,
		RetentionPolicy: i.RetentionPolicy,
		Consistency:     i.WriteConsistency,
		Serializer:      i.serializer,
	}

	c, err := i.CreateHTTPClientF(config)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client [%s]: %v", url, err)
	}

	if !i.SkipDatabaseCreation {
		err = c.CreateDatabase(ctx)
		if err != nil {
			log.Printf("W! [outputs.influxdb] when writing to [%s]: database %q creation failed: %v",
				c.URL(), c.Database(), err)
		}
	}

	return c, nil
}

func newInflux() *Orangesys {
	return &Orangesys{
		Timeout: internal.Duration{Duration: time.Second * 5},
		CreateHTTPClientF: func(config *HTTPConfig) (Client, error) {
			return NewHTTPClient(config)
		},
	}
}

func init() {
	outputs.Add("orangesys", func() telegraf.Output { return newInflux() })
}
