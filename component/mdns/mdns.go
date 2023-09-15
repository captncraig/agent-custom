package mdns

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/grafana/agent/component"
	"github.com/grafana/agent/component/discovery"
	"github.com/grafana/agent/service/http"
	dns "github.com/hashicorp/mdns"
)

func init() {
	component.Register(component.Registration{
		Name:          "discovery.mdns",
		Args:          Arguments{},
		Exports:       discovery.Exports{},
		NeedsServices: []string{http.ServiceName},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Service             string        `river:"service,attr"`               // Service to lookup
	Domain              string        `river:"domain,attr,optional"`       // Lookup domain, default "local"
	Timeout             time.Duration `river:"timeout,attr,optional"`      // Lookup timeout, default 1 second
	WantUnicastResponse bool          `river:"want_unicast,attr,optional"` // Unicast response desired, as per 5.4 in RFC
	DisableIPv4         bool          `river:"disable_ipv4,attr,optional"` // Whether to disable usage of IPv4 for MDNS operations. Does not affect discovered addresses.
	DisableIPv6         bool          `river:"disable_ipv6,attr,optional"` // Whether to disable usage of IPv6 for MDNS operations. Does not affect discovered addresses.
	RefreshInterval     time.Duration `river:"refresh_interval,attr,optional"`
}

var DefaultArguments = Arguments{
	Domain:          "local",
	Timeout:         time.Second,
	RefreshInterval: 5 * time.Second,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	return nil
}

func (args *Arguments) Convert() *dns.QueryParam {
	return &dns.QueryParam{
		Service:             args.Service,
		Domain:              args.Domain,
		Timeout:             args.Timeout,
		WantUnicastResponse: args.WantUnicastResponse,
		DisableIPv4:         args.DisableIPv4,
		DisableIPv6:         args.DisableIPv6,
	}
}

type Component struct {
	opts component.Options

	mut         sync.Mutex
	args        Arguments
	argsUpdated chan struct{}
}

func New(o component.Options, args component.Arguments) (*Component, error) {
	c := &Component{
		opts:        o,
		argsUpdated: make(chan struct{}, 1),
	}
	return c, c.Update(args)
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	var currentArgs Arguments
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.nextPoll(last, currentArgs.RefreshInterval)):
			c.poll(currentArgs)
			last = time.Now()
		case <-c.argsUpdated:
			c.mut.Lock()
			currentArgs = c.args
			c.mut.Unlock()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.mut.Lock()
	c.args = newArgs
	select {
	case c.argsUpdated <- struct{}{}:
	default:
	}
	c.mut.Unlock()
	return nil
}

// nextPoll returns how long to wait to poll given the last time a
// poll occurred. nextPoll returns 0 if a poll should occur immediately.
func (c *Component) nextPoll(last time.Time, interval time.Duration) time.Duration {
	nextPoll := last.Add(interval)
	now := time.Now()

	if now.After(nextPoll) {
		// Poll immediately; next poll period was in the past.
		return 0
	}
	return nextPoll.Sub(now)
}

func (c *Component) poll(args Arguments) {
	qp := args.Convert()
	ch := make(chan *dns.ServiceEntry)
	qp.Entries = ch
	go func() {
		dns.Query(qp)
		close(qp.Entries)
	}()
	for entry := range ch {
		log.Println(entry)
	}
}
