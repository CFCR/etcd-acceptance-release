package main_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	turbclient "github.com/jfmyers9/turbulence/client"
	"github.com/jfmyers9/turbulence/incident"
	"github.com/jfmyers9/turbulence/incident/selector"
	"github.com/jfmyers9/turbulence/tasks"
)

var _ = Describe("Experiment Two", func() {
	var turbClient turbclient.Turbulence

	BeforeEach(func() {
		turbClient = buildTurbulenceClient(cfg)
	})

	AfterEach(func() {
		// This repairs the cluster from the failing state
		By("Repairing the Cluster")
		restartEtcdNode(cfg.DeploymentName, "etcd", "0", director)

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

	It("maintains uptime through network partition and job restarts", func() {
		By("Creating the measurer")
		measurer, err := NewUptimeMeasurer(client, time.Second)
		Expect(err).NotTo(HaveOccurred())

		By("Starting the measurer")
		measurer.Start()

		By("Isolating the ETCD 0 (z1) node")
		clientIP := findClientIP("etcd-acceptance", director)
		unblockIP(cfg.DeploymentName, "etcd", "0", clientIP, director)
		isolatedNodeIncident := createNodeIncident(turbClient, cfg.DeploymentName)
		Expect(isolatedNodeIncident.HasTaskErrors()).To(BeFalse())

		By("Restarting ETCD 0 (z1)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "0", director)

		By("Restarting ETCD 1 (z2)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "1", director)

		By("Restarting ETCD 2 (z3)")
		restartEtcdNode(cfg.DeploymentName, "etcd", "2", director)

		By("Reconnecting the ETCD 0 (z1) node")
		for _, task := range isolatedNodeIncident.TasksOfType(tasks.FirewallOptions{}) {
			task.Stop()
		}
		isolatedNodeIncident.Wait()
		Expect(isolatedNodeIncident.HasTaskErrors()).To(BeFalse())
		cleanupIptables(cfg.DeploymentName, "etcd", "0", director)

		By("Stopping the measurer")
		Expect(measurer.Stop()).To(Succeed())

		By("Fetching the measurer's counts")
		total, failed := measurer.Counts()
		Expect(total).To(BeNumerically(">", 0), "No reads undertaken")
		actualDeviation := measurer.ActualDeviation()
		By(fmt.Sprintf("Calculating the deviation of failures: total: %d, failed: %d, deviation: %.5f", total, failed, actualDeviation))
		Expect(actualDeviation).To(BeNumerically("<=", cfg.ReadTolerance))
	})
})

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

func createNodeIncident(turbClient turbclient.Turbulence, deployment string) turbclient.Incident {
	req := incident.Request{
		Tasks: tasks.OptionsSlice{
			tasks.FirewallOptions{},
		},
		Selector: selector.Request{
			Deployment: &selector.NameRequest{Name: deployment},
			AZ:         &selector.NameRequest{Name: "z1"},
		},
	}

	return turbClient.CreateIncident(req)
}

func restartEtcdNode(deployment, instanceGroup, index string, director boshdir.Director) {
	host, username, privateKey, err := getSSHCreds(deployment, instanceGroup, index, director)
	Expect(err).NotTo(HaveOccurred())
	_, err = runSSHCommand(host, 22, username, privateKey, "sudo /var/vcap/bosh/bin/monit restart etcd")
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() string {
		output, err := runSSHCommand(host, 22, username, privateKey, "sudo /var/vcap/bosh/bin/monit summary")
		Expect(err).NotTo(HaveOccurred())
		return output
	}, time.Minute).Should(MatchRegexp(`'etcd'\s+running`))

	Expect(cleanupSSHCreds(deployment, instanceGroup, index, director)).To(Succeed())
}

func unblockIP(deployment, instanceGroup, index, targetIP string, director boshdir.Director) {
	host, username, privateKey, err := getSSHCreds(deployment, instanceGroup, index, director)
	Expect(err).NotTo(HaveOccurred())
	_, err = runSSHCommand(host, 22, username, privateKey, fmt.Sprintf("sudo iptables -I INPUT 1 -s %s -j ACCEPT && sudo iptables -I OUTPUT 1 -s %s -j ACCEPT", targetIP, targetIP))
	Expect(err).NotTo(HaveOccurred())
	Expect(cleanupSSHCreds(deployment, instanceGroup, index, director)).To(Succeed())
}

func cleanupIptables(deployment, instanceGroup, index string, director boshdir.Director) {
	host, username, privateKey, err := getSSHCreds(deployment, instanceGroup, index, director)
	Expect(err).NotTo(HaveOccurred())
	_, err = runSSHCommand(host, 22, username, privateKey, "sudo iptables -D INPUT 1 && sudo iptables -D OUTPUT 1")
	Expect(err).NotTo(HaveOccurred())
	Expect(cleanupSSHCreds(deployment, instanceGroup, index, director)).To(Succeed())
}

func findClientIP(acceptanceDeployment string, director boshdir.Director) string {
	dep, err := director.FindDeployment(acceptanceDeployment)
	Expect(err).NotTo(HaveOccurred())
	infos, err := dep.VMInfos()
	Expect(err).NotTo(HaveOccurred())
	Expect(infos).To(HaveLen(1))
	Expect(infos[0].IPs).To(HaveLen(1))
	return infos[0].IPs[0]
}
