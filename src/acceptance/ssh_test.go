package main_test

import (
	"bytes"
	"fmt"
	"log"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/cloudfoundry/bosh-utils/errors"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	"golang.org/x/crypto/ssh"
)

func runSSHCommand(server string, port int, username string, privateKey string, command string) (string, error) {
	parsedPrivateKey, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		log.Println(err)
		return "", err
	}

	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(parsedPrivateKey),
		},
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server, port), config)
	if err != nil {
		return "", errors.WrapError(err, "Cannot dial")
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return "", errors.WrapError(err, "Cannot create session")
	}
	defer session.Close()

	var output bytes.Buffer

	session.Stdout = &output

	err = session.Run(command)
	if err != nil {
		return "", errors.WrapError(err, "Failed running command")
	}

	return output.String(), nil
}

func getSSHCreds(
	deploymentName, instanceGroupName, index string,
	director boshdir.Director,
) (string, string, string, error) {
	deployment, err := director.FindDeployment(deploymentName)
	if err != nil {
		return "", "", "", err
	}

	sshOpts, privateKey, err := boshdir.NewSSHOpts(boshuuid.NewGenerator())
	if err != nil {
		return "", "", "", err
	}

	slug := boshdir.NewAllOrInstanceGroupOrInstanceSlug(instanceGroupName, index)
	sshResult, err := deployment.SetUpSSH(slug, sshOpts)
	if err != nil {
		return "", "", "", err
	}

	return sshResult.Hosts[0].Host, sshOpts.Username, privateKey, nil
}

func cleanupSSHCreds(
	deploymentName, instanceGroupName, index string,
	director boshdir.Director,
) error {
	deployment, err := director.FindDeployment(deploymentName)
	if err != nil {
		return err
	}

	sshOpts, _, err := boshdir.NewSSHOpts(boshuuid.NewGenerator())
	if err != nil {
		return err
	}

	slug := boshdir.NewAllOrInstanceGroupOrInstanceSlug(instanceGroupName, index)
	return deployment.CleanUpSSH(slug, sshOpts)
}
