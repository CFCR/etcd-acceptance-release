package main_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	turbclient "github.com/jfmyers9/turbulence/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const etcdOperationTimeout = 10 * time.Second

var (
	configPath string
	cfg        config

	director   boshdir.Director
	client     *clientv3.Client
	turbClient turbclient.Turbulence
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
		CertFile: cfg.ClientCertPath,
		KeyFile:  cfg.ClientKeyPath,
		CAFile:   cfg.ClientCAPath,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	Expect(err).NotTo(HaveOccurred())

	client, err = clientv3.New(clientv3.Config{
		DialKeepAliveTime:    30 * time.Second,
		DialKeepAliveTimeout: 10 * time.Second,
		Endpoints:            cfg.Endpoints,
		TLS:                  tlsConfig,
	})
	Expect(err).NotTo(HaveOccurred())

	director, err = buildDirector(cfg)
	Expect(err).NotTo(HaveOccurred())

	turbClient = buildTurbulenceClient(cfg)
})

var _ = AfterEach(func() {
	By("Waiting for all nodes to be running")
	cleanupIptables(cfg.DeploymentName, "etcd", "0", director)
	cleanupIptables(cfg.DeploymentName, "etcd", "1", director)
	cleanupIptables(cfg.DeploymentName, "etcd", "2", director)

	Eventually(func() error {
		dep, err := director.FindDeployment(cfg.DeploymentName)
		Expect(err).NotTo(HaveOccurred())

		vms, err := dep.VMInfos()
		Expect(err).NotTo(HaveOccurred())

		for _, vm := range vms {
			if vm.ProcessState != "running" {
				return fmt.Errorf("Not all VMs Running (%s/%s)", vm.JobName, vm.ID)
			}
		}

		return nil
	}, 3*time.Minute).ShouldNot(HaveOccurred())
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
	DeploymentName       string `json:"deployment_name"`

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

func buildTurbulenceClient(cfg config) turbclient.Turbulence {
	logger := boshlog.NewLogger(boshlog.LevelNone)

	turbCfg := turbclient.Config{
		Host:     cfg.TurbulenceHost,
		Port:     cfg.TurbulencePort,
		Username: cfg.TurbulenceUser,
		Password: cfg.TurbulencePassword,
		CACert:   cfg.TurbulenceCACert,
	}

	return turbclient.NewFactory(logger).New(turbCfg)
}
