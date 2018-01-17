package main_test

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("Acceptance", func() {
	var (
		director boshdir.Director
		measurer *uptimeMeasurer
	)

	BeforeEach(func() {
		By("Creating the dependencies")
		tlsInfo := transport.TLSInfo{
			CertFile:      cfg.ClientCertPath,
			KeyFile:       cfg.ClientKeyPath,
			TrustedCAFile: cfg.ClientCAPath,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		Expect(err).NotTo(HaveOccurred())

		client, err := clientv3.New(clientv3.Config{
			Endpoints:   cfg.Endpoints,
			DialTimeout: 5 * time.Second,
			TLS:         tlsConfig,
		})
		Expect(err).NotTo(HaveOccurred())

		director, err = buildDirector(cfg)
		Expect(err).NotTo(HaveOccurred())

		guid := uuid.NewV4()
		key := fmt.Sprintf("test-key-%s", guid.String())
		value := fmt.Sprintf("test-value-%s", guid.String())

		By("Launching the measurer")
		measurer = NewUptimeMeasurer(client)
		Expect(measurer.Start(key, value)).To(Succeed())
	})

	AfterEach(func() {
		By("Stopping the measurer")
		measurer.Stop()

		By("Fetching the measurer's counts")
		total, failed := measurer.Counts()

		Expect(total).To(BeNumerically(">", 0), "No reads undertaken")
		actualDeviation := float64(failed) / float64(total)

		By(fmt.Sprintf("Calculating the deviation of failures: total: %d, failed: %d, deviation: %.5f\n", total, failed, actualDeviation))
		Expect(actualDeviation).To(BeNumerically("<=", cfg.ReadTolerance))
	})

	It("maintains uptime through a bosh recreate", func() {
		By("Recreating the deployment")
		deployment, err := director.FindDeployment("etcd")
		Expect(err).NotTo(HaveOccurred())
		Expect(deployment.Recreate(boshdir.AllOrInstanceGroupOrInstanceSlug{}, boshdir.RecreateOpts{})).To(Succeed())
	})
})

type uptimeMeasurer struct {
	failedCount int
	totalCount  int

	cancelled chan struct{}
	stopped   chan struct{}

	client *clientv3.Client
}

func NewUptimeMeasurer(client *clientv3.Client) *uptimeMeasurer {
	return &uptimeMeasurer{
		client:    client,
		cancelled: make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

func (u *uptimeMeasurer) Start(key, value string) error {
	By("Starting the measurer")
	_, err := u.client.Put(context.Background(), key, value)
	if err != nil {
		close(u.stopped)
		return err
	}

	go func() {
		timer := time.NewTimer(time.Second)
		for {
			timer.Reset(time.Second)

			select {
			case <-u.cancelled:
				close(u.stopped)
				return
			case <-timer.C:
				u.totalCount++
				resp, err := u.client.Get(context.Background(), key)
				if err != nil {
					u.failedCount++
					continue
				}

				if len(resp.Kvs) != 1 {
					u.failedCount++
					continue
				}

				for _, kv := range resp.Kvs {
					if string(kv.Key) != key || string(kv.Value) != value {
						u.failedCount++
						break
					}
				}
			}
		}
	}()

	return nil
}

func (u *uptimeMeasurer) Stop() {
	close(u.cancelled)
}

func (u uptimeMeasurer) Counts() (int, int) {
	<-u.stopped
	return u.totalCount, u.failedCount
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
