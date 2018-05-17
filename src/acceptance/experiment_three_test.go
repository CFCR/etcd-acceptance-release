package main_test

import (
	"time"

	"code.cloudfoundry.org/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Experiment Three", func() {
	It("measures read availability and consistency through a complete network partition and its partial restoration", func() {
		By("Creating the initial state measurer")
		initialMeasurer, err := NewUptimeMeasurer(client, time.Second)
		Expect(err).NotTo(HaveOccurred())
		defer cleanupMeasurer(initialMeasurer)

		By("Starting the measurer")
		initialMeasurer.Start()

		By("Checking the initial state of the cluster")
		time.Sleep(60 * time.Second)

		By("Stopping the initial measurer")
		initialMeasurer.Stop()
		measurerExpectations(initialMeasurer, "<=", cfg.ReadTolerance, deadlineErrorsTolerance)

		By("Creating the total network partition measurer")
		totalNetworkPartitionMeasurer, err := NewUptimeMeasurer(client, time.Second)
		Expect(err).NotTo(HaveOccurred())
		defer cleanupMeasurer(totalNetworkPartitionMeasurer)

		clientIP, err := localip.LocalIP()
		Expect(err).NotTo(HaveOccurred())

		By("Isolating the ETCD 0 (z1) node")
		unblockIP(cfg.DeploymentName, "etcd", "0", clientIP, director)
		isolateZoneOneIncident := createNodeIncident(turbClient, cfg.DeploymentName, "z1")
		Expect(isolateZoneOneIncident.HasTaskErrors()).To(BeFalse())

		By("Isolating the ETCD 1 (z2) node")
		unblockIP(cfg.DeploymentName, "etcd", "1", clientIP, director)
		isolateZoneTwoIncident := createNodeIncident(turbClient, cfg.DeploymentName, "z2")
		Expect(isolateZoneTwoIncident.HasTaskErrors()).To(BeFalse())

		By("Waiting for reads to start failing")
		Eventually(readRootKey(client), 3*etcdOperationTimeout).Should(HaveOccurred())

		By("Starting the total network partition measurer")
		totalNetworkPartitionMeasurer.Start()

		By("Checking the state of the cluster")
		time.Sleep(60 * time.Second)

		By("Stopping the total network partition measurer")
		totalNetworkPartitionMeasurer.Stop()
		measurerExpectations(totalNetworkPartitionMeasurer, ">=", 1.00, deadlineErrorsTolerance)

		By("Lifting the partition on the ETCD 1 (z2) node")
		unIsolateNode(isolateZoneTwoIncident, cfg.DeploymentName, "1", director)

		By("Waiting for reads to start succeeding")
		Eventually(readRootKey(client), 3*etcdOperationTimeout).ShouldNot(HaveOccurred())

		By("Starting the partial network partition measurer")
		partialNetworkPartitionMeasurer, err := NewUptimeMeasurer(client, time.Second)
		Expect(err).NotTo(HaveOccurred())
		defer cleanupMeasurer(partialNetworkPartitionMeasurer)

		partialNetworkPartitionMeasurer.Start()

		By("Checking the state of the cluster")
		time.Sleep(60 * time.Second)
		measurerExpectations(partialNetworkPartitionMeasurer, "<=", cfg.ReadTolerance, deadlineErrorsTolerance)

		By("Lifting the partition on the ETCD 0 (z1) node")
		unIsolateNode(isolateZoneOneIncident, cfg.DeploymentName, "0", director)

		By("Checking the state of the cluster")
		time.Sleep(60 * time.Second)

		By("Stopping the total network partition measurer")
		partialNetworkPartitionMeasurer.Stop()
		measurerExpectations(partialNetworkPartitionMeasurer, "<=", cfg.ReadTolerance, deadlineErrorsTolerance)
	})
})
