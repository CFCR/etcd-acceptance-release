package main_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
)

var _ = Describe("Experiment One", func() {
	It("maintains uptime through a bosh recreate", func() {
		By("Launching the measurer")
		measurer, err := NewUptimeMeasurer(client, time.Second)
		Expect(err).NotTo(HaveOccurred())
		measurer.Start()

		By("Recreating the deployment")
		deployment, err := director.FindDeployment("etcd")
		Expect(err).NotTo(HaveOccurred())
		Expect(deployment.Recreate(boshdir.AllOrInstanceGroupOrInstanceSlug{}, boshdir.RecreateOpts{})).To(Succeed())

		By("Stopping the measurer")
		Expect(measurer.Stop()).To(Succeed())

		By("Fetching the measurer's counts")
		total, failed := measurer.Counts()
		Expect(total).To(BeNumerically(">", 0), "No reads undertaken")
		actualDeviation := measurer.ActualDeviation()
		By(fmt.Sprintf("Calculating the deviation of failures: total: %d, failed: %d, deviation: %.5f\n", total, failed, actualDeviation))
		Expect(actualDeviation).To(BeNumerically("<=", cfg.ReadTolerance))
	})
})
