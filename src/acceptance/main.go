package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
)

var configPath = flag.String("config", "", "path to config file")

func main() {
	flag.Parse()

	cfg, err := parseConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	tlsInfo := transport.TLSInfo{
		CertFile:      cfg.ClientCertPath,
		KeyFile:       cfg.ClientKeyPath,
		TrustedCAFile: cfg.ClientCAPath,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		log.Fatal(err)
	}

	key := "example"
	value := "example-value"

	_, err = client.Put(context.Background(), key, value)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Get(context.Background(), key)
	if err != nil {
		log.Fatal(err)
	}

	for _, kv := range resp.Kvs {
		log.Printf("%q key has %q value\n", kv.Key, kv.Value)
	}
}

type config struct {
	Endpoints      []string `json:"endpoints"`
	ClientCertPath string   `json:"client_cert_path"`
	ClientKeyPath  string   `json:"client_key_path"`
	ClientCAPath   string   `json:"client_ca_path"`
}

func parseConfig(path string) (config, error) {
	f, err := os.Open(path)
	if err != nil {
		return config{}, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)

	cfg := config{}
	err = decoder.Decode(&cfg)
	if err != nil {
		return config{}, err
	}

	return cfg, nil
}
