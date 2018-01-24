package main_test

import (
	"encoding/json"
	"flag"
	"os"
	"testing"
	"time"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	configPath string
	cfg        config

	director boshdir.Director
	client   *clientv3.Client
)

func init() {
	flag.StringVar(&configPath, "config", "", "path to test config")
}

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var _ = BeforeSuite(func() {
	flag.Parse()
	Expect(configPath).To(BeAnExistingFile())

	f, err := os.Open(configPath)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	decoder := json.NewDecoder(f)
	Expect(decoder.Decode(&cfg)).To(Succeed())

	tlsInfo := transport.TLSInfo{
		CertFile:      cfg.ClientCertPath,
		KeyFile:       cfg.ClientKeyPath,
		TrustedCAFile: cfg.ClientCAPath,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	Expect(err).NotTo(HaveOccurred())

	client, err = clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	Expect(err).NotTo(HaveOccurred())

	director, err = buildDirector(cfg)
	Expect(err).NotTo(HaveOccurred())
})

type config struct {
	ClientCAPath   string   `json:"client_ca_path"`
	ClientCertPath string   `json:"client_cert_path"`
	ClientKeyPath  string   `json:"client_key_path"`
	Endpoints      []string `json:"endpoints"`

	ReadTolerance float64 `json:"read_tolerance"`

	DirectorCA           string `json:"director_ca"`
	DirectorClient       string `json:"director_client"`
	DirectorClientSecret string `json:"director_client_secret"`
	DirectorURL          string `json:"director_url"`

	TurbulenceHost     string `json:"turbulence_host"`
	TurbulencePort     int    `json:"turbulence_port"`
	TurbulenceUser     string `json:"turbulence_user"`
	TurbulencePassword string `json:"turbulence_password"`
	TurbulenceCACert   string `json:"turbulence_ca_cert"`

	UAAURL string `json:"uaa_url"`
}

func buildDirector(cfg config) (boshdir.Director, error) {
	logger := boshlog.NewLogger(boshlog.LevelError)
	uaaFactory := boshuaa.NewFactory(logger)

	uaaCfg, err := boshuaa.NewConfigFromURL(cfg.UAAURL)
	if err != nil {
		return nil, err
	}

	uaaCfg.Client = cfg.DirectorClient
	uaaCfg.ClientSecret = cfg.DirectorClientSecret
	uaaCfg.CACert = cfg.DirectorCA

	uaa, err := uaaFactory.New(uaaCfg)
	if err != nil {
		return nil, err
	}

	directorFactory := boshdir.NewFactory(logger)

	directorCfg, err := boshdir.NewConfigFromURL(cfg.DirectorURL)
	Expect(err).NotTo(HaveOccurred())

	directorCfg.CACert = cfg.DirectorCA
	directorCfg.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc

	return directorFactory.New(directorCfg, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
}
