package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/patrickmn/go-cache"
)

func pidToContainer(pid int) (string, error) {

	file, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		line := strings.Trim(scanner.Text(), " ")

		tokens := strings.SplitN(line, ":", 3)

		if strings.Compare(tokens[1], "cpuset") == 0 && strings.HasPrefix(tokens[2], "/docker/") {
			return strings.TrimPrefix(tokens[2], "/docker/"), nil
		}

	}

	return "", errors.New("process does not belong to a container")
}

func connectDocker() *client.Client {
	client, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	return client
}

var dockerContext context.Context = context.Background()

var dockerClient *client.Client = connectDocker()

var userCache *cache.Cache = cache.New(5*time.Minute, 10*time.Minute)
var networkCache *cache.Cache = cache.New(5*time.Minute, 10*time.Minute)

func findContainer(address net.Addr) (string, error) {

	list, err := dockerClient.ContainerList(dockerContext, types.ContainerListOptions{})

	if err != nil {
		return "", err
	}

	for _, container := range list {

		for _, network := range container.NetworkSettings.Networks {
			if strings.Compare(network.IPAddress, address.String()) == 0 {
				return container.ID, nil
			}
		}
	}

	return "", nil

}

func identifyProcess(pid int) (string, error) {

	container, err := pidToContainer(pid)

	if err != nil {
		return "", err
	}

	return findOwner(container)

}

func findOwner(containerID string) (string, error) {

	labels := [...]string{"ccc-user.email", "user.email", "email", "maintainer"}

	user, found := userCache.Get(containerID)

	if found {
		return user.(string), nil
	}

	list, err := dockerClient.ContainerList(dockerContext, types.ContainerListOptions{})

	if err != nil {
		return "", err
	}

	for _, container := range list {
		if strings.Compare(container.ID, containerID) == 0 {

			for name, value := range container.Labels {

				for _, label := range labels {

					if strings.Compare(label, name) == 0 {

						address, err := mail.ParseAddress(value)

						if err == nil {

							user := address.Address

							userCache.SetDefault(containerID, user)

							return user, nil

						}

					}

				}

			}

		}
	}

	return "", errors.New("user not found")

}

func identifyAddress(address net.Addr) (string, error) {
	container, err := findContainer(address)

	if err != nil {
		return "", nil
	}

	owner, err := findOwner(container)

	return owner, err

}
