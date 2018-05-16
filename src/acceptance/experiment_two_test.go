package main_test

import (
	"time"

	"code.cloudfoundry.org/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Experiment Two", func() {
	It("maintains uptime through network partition and job restarts", func() {
		By("Creating the measurer")
		measurer, err := NewUptimeMeasurer(client, 500*time.Millisecond)
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
		Expect(measurer.UpdateValidKeyValue()).NotTo(HaveOccurred())

		By("Restarting ETCD 0 (z1)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "0", director)

		By("Restarting ETCD 1 (z2)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "1", director)

		By("Restarting ETCD 2 (z3)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "2", director)

		By("Reconnecting the ETCD 0 (z1) node")
		unIsolateNode(isolatedNodeIncident, cfg.DeploymentName, "0", director)

		By("Stopping the measurer")
		measurer.Stop()

		By("Fetching the measurer's counts")
		measurerExpectations(measurer, "<=", cfg.ReadTolerance)
	})
})
