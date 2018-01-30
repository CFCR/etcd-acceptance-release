package main_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/localip"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Experiment Four", func() {
	var (
		client0     *clientv3.Client
		client1And2 *clientv3.Client
	)

	var buildTargetedETCDClient = func(cfg config, start, stop int) *clientv3.Client {
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
			Endpoints:            cfg.Endpoints[start:stop],
			TLS:                  tlsConfig,
		})
		Expect(err).NotTo(HaveOccurred())

		return client
	}

	BeforeEach(func() {
		client0 = buildTargetedETCDClient(cfg, 0, 1)
		fmt.Printf("Client 0 Endpoints: %#v\n", client0.Endpoints())
		client1And2 = buildTargetedETCDClient(cfg, 1, 3)
		fmt.Printf("Client 1 and 2 Endpoints: %#v\n", client1And2.Endpoints())
	})

	It("maintains uptime through network partition and job restarts with targeted clients", func() {
		By("Creating the measurer")
		measurer, err := NewUptimeMeasurer(client1And2, time.Second)
		Expect(err).NotTo(HaveOccurred())
		defer cleanupMeasurer(measurer)

		By("Starting the measurer")
		measurer.Start()

		By("Isolating the ETCD 0 (z1) node")
		clientIP, err := localip.LocalIP()
		Expect(err).NotTo(HaveOccurred())
		unblockIP(cfg.DeploymentName, "etcd", "0", clientIP, director)
		isolatedNodeIncident := createNodeIncident(turbClient, cfg.DeploymentName, "z1")
		Expect(isolatedNodeIncident.HasTaskErrors()).To(BeFalse())

		Eventually(readRootKey(client0), 3*etcdOperationTimeout).Should(HaveOccurred())
		Consistently(readRootKey(client0), 3*etcdOperationTimeout).Should(HaveOccurred())

		By("Restarting ETCD 0 (z1)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "0", director)

		Consistently(readRootKey(client0), 3*etcdOperationTimeout).Should(HaveOccurred())

		By("Restarting ETCD 1 (z2)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "1", director)

		Consistently(readRootKey(client0), 3*etcdOperationTimeout).Should(HaveOccurred())

		By("Restarting ETCD 2 (z3)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "2", director)

		Consistently(readRootKey(client0), 3*etcdOperationTimeout).Should(HaveOccurred())

		By("Reconnecting the ETCD 0 (z1) node")
		unIsolateNode(isolatedNodeIncident, cfg.DeploymentName, "0", director)

		Eventually(readRootKey(client0), 3*etcdOperationTimeout).ShouldNot(HaveOccurred())

		By("Stopping the measurer")
		measurer.Stop()

		By("Fetching the measurer's counts")
		measurerExpectations(measurer, "<=", cfg.ReadTolerance)
	})
})
