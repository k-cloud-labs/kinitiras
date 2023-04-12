package options

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	defaultBindAddress   = "0.0.0.0"
	defaultPort          = 8443
	defaultCertDir       = "/tmp/k8s-webhook-server/serving-certs"
	defaultTLSMinVersion = "1.3"
)

// Options contains everything necessary to create and run webhook server.
type Options struct {
	// BindAddress is the IP address on which to listen for the --secure-port port.
	// Default is "0.0.0.0".
	BindAddress string
	// SecurePort is the port that the webhook server serves at.
	// Default is 8443.
	SecurePort int
	// MetricsBindAddress is the IP:Port address on which to listen for the webhook metrics.
	// Default is ":8080".
	MetricsBindAddress string
	// CertDir is the directory that contains the server key and certificate.
	// if not set, webhook server would look up the server key and certificate in {TempDir}/k8s-webhook-server/serving-certs.
	// The server key and certificate must be named `tls.key` and `tls.crt`, respectively.
	CertDir string
	// TLSMinVersion is the minimum version of TLS supported. Possible values: 1.0, 1.1, 1.2, 1.3.
	// Some environments have automated security scans that trigger on TLS versions or insecure cipher suites, and
	// setting TLS to 1.3 would solve both problems.
	// Defaults to 1.3.
	TLSMinVersion string
	// KubeAPIQPS is the QPS to use while talking with kube-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with kube-apiserver.
	KubeAPIBurst int
	// PreCacheResources is a list of resources name to pre-cache when start up.
	PreCacheResources *ResourceSlice
	// EnablePProf is switch to enable/disable net/http/pprof. Default value as false.
	EnablePProf bool
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.PreCacheResources = NewPreCacheResources([]string{})
	flags.StringVar(&o.BindAddress, "bind-address", defaultBindAddress,
		"The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.SecurePort, "secure-port", defaultPort,
		"The secure port on which to serve HTTPS.")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", metrics.DefaultBindAddress,
		"The Metrics bind address on which to listen for the webhook metrics.")
	flags.StringVar(&o.CertDir, "cert-dir", defaultCertDir,
		"The directory that contains the server key(named tls.key) and certificate(named tls.crt).")
	flags.StringVar(&o.TLSMinVersion, "tls-min-version", defaultTLSMinVersion, "Minimum TLS version supported. Possible values: 1.0, 1.1, 1.2, 1.3.")
	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with kube-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with kube-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.VarP(o.PreCacheResources, "pre-cache-resources", "", "Resources list separate by comma, for example: Pod/v1,Deployment/apps/v1"+
		". Will pre cache those resources to get it quicker when policies refer resources from cluster.")
	flags.BoolVar(&o.EnablePProf, "enable-pprof", false, "EnablePProf is switch to enable/disable net/http/pprof. Default value as false.")

	globalflag.AddGlobalFlags(flags, "global")
}

// PrintFlags logs the flags in the flagset
func PrintFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		klog.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
}

type ResourceSlice struct {
	value   *[]schema.GroupVersionKind
	changed bool
}

func NewPreCacheResources(slice []string) *ResourceSlice {
	value := make([]schema.GroupVersionKind, 0)
	s := &ResourceSlice{value: &value}
	_ = s.Replace(slice)
	return s
}

func (s *ResourceSlice) String() string {
	sb := &strings.Builder{}

	// check if the value is nil
	if s.value == nil {
		return "[]"
	}

	for i, gvk := range *s.value {
		if i != 0 {
			sb.WriteString(",")
		}
		sb.WriteString(gvk.Kind + "/" + gvk.GroupVersion().String())
	}

	return "[" + sb.String() + "]"
}

func (s *ResourceSlice) Set(val string) error {
	if s.value == nil {
		return fmt.Errorf("no target (nil pointer to []string)")
	}
	if !s.changed {
		*s.value = make([]schema.GroupVersionKind, 0)
	}

	// split by comma
	vals := strings.Split(val, ",")
	for _, v := range vals {
		gvk, err := s.readResource(v)
		if err != nil {
			return err
		}
		*s.value = append(*s.value, gvk)
		s.changed = true
	}

	return nil
}

func (s *ResourceSlice) Type() string {
	return "resourceSlice"
}

func (s *ResourceSlice) Append(val string) error {
	gvk, err := s.readResource(val)
	if err != nil {
		return err
	}

	*s.value = append(*s.value, gvk)
	return nil
}

func (s *ResourceSlice) Replace(slice []string) error {
	value := make([]schema.GroupVersionKind, 0, len(slice))
	for _, str := range slice {
		gvk, err := s.readResource(str)
		if err != nil {
			return err
		}

		value = append(value, gvk)
	}

	*s.value = value
	return nil
}

func (s *ResourceSlice) GetSlice() []string {
	var slice = make([]string, 0, len(*s.value))
	for _, gvk := range *s.value {
		slice = append(slice, gvk.Kind+"/"+gvk.GroupVersion().String())
	}

	return slice
}

func (s *ResourceSlice) readResource(val string) (schema.GroupVersionKind, error) {
	var gvk schema.GroupVersionKind
	items := strings.Split(val, "/")
	if len(items) <= 1 {
		return gvk, fmt.Errorf("invalid gvk(%v)", val)
	}

	gvk.Kind = items[0]
	gvk.Version = items[1]

	if len(items) == 3 {
		gvk.Group = items[1]
		gvk.Version = items[2]
	}

	return gvk, nil
}

func (o *Options) PreCacheResourcesToGVKList() []schema.GroupVersionKind {
	return *o.PreCacheResources.value
}
