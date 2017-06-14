package orangesys

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/plugins/outputs"

	"github.com/influxdata/telegraf/plugins/outputs/orangesys/client"
)

var (
	// Quote Ident replacer.
	qiReplacer = strings.NewReplacer("\n", `\n`, `\`, `\\`, `"`, `\"`)
)

// Orangesys struct is the primary data structure for the plugin
type Orangesys struct {
	// URL is only for backwards compatability
	URL              string
	URLs             []string `toml:"urls"`
	Username         string
	Password         string
	JwtToken         string `toml:"jwt_token"`
	Database         string
	UserAgent        string
	RetentionPolicy  string
	WriteConsistency string
	Timeout          internal.Duration
	UDPPayload       int `toml:"udp_payload"`

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

	clients []client.Client
}

var sampleConfig = `
  ## The HTTP or UDP URL for your Orangesys instance.  Each item should be
  ## of the form:
  ##   scheme "://" host [ ":" port]
  ##
  ## Multiple urls can be specified as part of the same cluster,
  ## this means that only ONE of the urls will be written to each interval.
  # urls = ["udp://localhost:8089"] # UDP endpoint example
  urls = ["http://localhost:8086"] # required
  ## The target database for metrics (telegraf will create it if not exists).
  database = "telegraf" # required

  ## Name of existing retention policy to write to.  Empty string writes to
  ## the default retention policy.
  retention_policy = ""
  ## Write consistency (clusters only), can be: "any", "one", "quorum", "all"
  write_consistency = "any"

  ## Write timeout (for the Orangesys client), formatted as a string.
  ## If not provided, will default to 5s. 0s means no timeout (not recommended).
  timeout = "5s"
  # username = "telegraf"
  # password = "metricsmetricsmetricsmetrics"
  # jwt_token = "jwt_token"
  ## Set the user agent for HTTP POSTs (can be useful for log differentiation)
  # user_agent = "telegraf"
  ## Set UDP payload size, defaults to Orangesys UDP Client default (512 bytes)
  # udp_payload = 512

  ## Optional SSL Config
  # ssl_ca = "/etc/telegraf/ca.pem"
  # ssl_cert = "/etc/telegraf/cert.pem"
  # ssl_key = "/etc/telegraf/key.pem"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false
`

// Connect initiates the primary connection to the range of provided URLs
func (i *Orangesys) Connect() error {
	var urls []string
	urls = append(urls, i.URLs...)

	// Backward-compatability with single Influx URL config files
	// This could eventually be removed in favor of specifying the urls as a list
	if i.URL != "" {
		urls = append(urls, i.URL)
	}

	tlsConfig, err := internal.GetTLSConfig(
		i.SSLCert, i.SSLKey, i.SSLCA, i.InsecureSkipVerify)
	if err != nil {
		return err
	}

	for _, u := range urls {
		switch {
		case strings.HasPrefix(u, "udp"):
			config := client.UDPConfig{
				URL:         u,
				PayloadSize: i.UDPPayload,
			}
			c, err := client.NewUDP(config)
			if err != nil {
				return fmt.Errorf("Error creating UDP Client [%s]: %s", u, err)
			}
			i.clients = append(i.clients, c)
		default:
			// If URL doesn't start with "udp", assume HTTP client
			config := client.HTTPConfig{
				URL:       u,
				Timeout:   i.Timeout.Duration,
				TLSConfig: tlsConfig,
				UserAgent: i.UserAgent,
				Username:  i.Username,
				Password:  i.Password,
				JwtToken:  i.JwtToken,
			}
			wp := client.WriteParams{
				Database:        i.Database,
				RetentionPolicy: i.RetentionPolicy,
				Consistency:     i.WriteConsistency,
			}
			c, err := client.NewHTTP(config, wp)
			if err != nil {
				return fmt.Errorf("Error creating HTTP Client [%s]: %s", u, err)
			}
			i.clients = append(i.clients, c)

			err = c.Query(fmt.Sprintf(`CREATE DATABASE "%s"`, qiReplacer.Replace(i.Database)))
			if err != nil {
				if !strings.Contains(err.Error(), "Status Code [403]") {
					log.Println("I! Database creation failed: " + err.Error())
				}
				continue
			}
		}
	}

	rand.Seed(time.Now().UnixNano())
	return nil
}

// Close will terminate the session to the backend, returning error if an issue arises
func (i *Orangesys) Close() error {
	return nil
}

// SampleConfig returns the formatted sample configuration for the plugin
func (i *Orangesys) SampleConfig() string {
	return sampleConfig
}

// Description returns the human-readable function definition of the plugin
func (i *Orangesys) Description() string {
	return "Configuration for orangesys server to send metrics to"
}

// Write will choose a random server in the cluster to write to until a successful write
// occurs, logging each unsuccessful. If all servers fail, return error.
func (i *Orangesys) Write(metrics []telegraf.Metric) error {

	bufsize := 0
	for _, m := range metrics {
		bufsize += m.Len()
	}

	r := metric.NewReader(metrics)

	// This will get set to nil if a successful write occurs
	err := fmt.Errorf("Could not write to any Orangesys server in cluster")

	p := rand.Perm(len(i.clients))
	for _, n := range p {
		if _, e := i.clients[n].WriteStream(r, bufsize); e != nil {
			// If the database was not found, try to recreate it:
			if strings.Contains(e.Error(), "database not found") {
				errc := i.clients[n].Query(fmt.Sprintf(`CREATE DATABASE "%s"`, qiReplacer.Replace(i.Database)))
				if errc != nil {
					log.Printf("E! Error: Database %s not found and failed to recreate\n",
						i.Database)
				}
			}
			if strings.Contains(e.Error(), "field type conflict") {
				log.Printf("E! Field type conflict, dropping conflicted points: %s", e)
				// setting err to nil, otherwise we will keep retrying and points
				// w/ conflicting types will get stuck in the buffer forever.
				err = nil
				break
			}
			// Log write failure
			log.Printf("E! Orangesys Output Error: %s", e)
		} else {
			err = nil
			break
		}
	}

	return err
}

func newInflux() *Orangesys {
	return &Orangesys{
		Timeout: internal.Duration{Duration: time.Second * 5},
	}
}

func init() {
	outputs.Add("orangesys", func() telegraf.Output { return newInflux() })
}
