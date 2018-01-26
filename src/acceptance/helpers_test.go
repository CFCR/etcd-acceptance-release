package main_test

import (
	"context"
	"fmt"
	"time"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/coreos/etcd/clientv3"
	turbclient "github.com/jfmyers9/turbulence/client"
	"github.com/jfmyers9/turbulence/incident"
	"github.com/jfmyers9/turbulence/incident/selector"
	"github.com/jfmyers9/turbulence/tasks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func createNodeIncident(turbClient turbclient.Turbulence, deployment, zone string) turbclient.Incident {
	req := incident.Request{
		Tasks: tasks.OptionsSlice{
			tasks.FirewallOptions{},
		},
		Selector: selector.Request{
			Deployment: &selector.NameRequest{Name: deployment},
			AZ:         &selector.NameRequest{Name: zone},
		},
	}

	return turbClient.CreateIncident(req)
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

func measurerExpectations(measurer *uptimeMeasurer, comparisonOperator string, readTolerance float64) {
	total, failed := measurer.Counts()
	Expect(total).To(BeNumerically(">", 0), "No reads undertaken")
	actualDeviation := measurer.ActualDeviation()
	By(fmt.Sprintf("Calculating the deviation of failures: total: %d, failed: %d, deviation: %.5f", total, failed, actualDeviation))
	Expect(actualDeviation).To(BeNumerically(comparisonOperator, readTolerance))
}

func cleanupMeasurer(measurer *uptimeMeasurer) {
	Expect(measurer.Cleanup()).To(Succeed())
}

func unIsolateNode(incident turbclient.Incident, deployment, index string, director boshdir.Director) {
	for _, task := range incident.TasksOfType(tasks.FirewallOptions{}) {
		task.Stop()
	}
	incident.Wait()
	Expect(incident.HasTaskErrors()).To(BeFalse())
	cleanupIptables(deployment, "etcd", index, director)
}

func readRootKey(client *clientv3.Client) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), etcdOperationTimeout)
		defer cancel()

		_, err := client.Get(ctx, "/")
		return err
	}
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
