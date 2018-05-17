package main_test

import (
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
		defer cleanupMeasurer(measurer)

		measurer.Start()

		By("Recreating the deployment")
		deployment, err := director.FindDeployment(cfg.DeploymentName)
		Expect(err).NotTo(HaveOccurred())
		Expect(deployment.Recreate(boshdir.AllOrInstanceGroupOrInstanceSlug{}, boshdir.RecreateOpts{})).To(Succeed())

		By("Stopping the measurer")
		measurer.Stop()

		By("Fetching the measurer's counts")
		measurerExpectations(measurer, "<=", cfg.ReadTolerance, deadlineErrorsTolerance)
	})
})
