package main_test

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	configPath string
	cfg        config
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

	UAAURL string `json:"uaa_url"`
}
