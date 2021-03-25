package main

// https://github.com/olekukonko/tablewriter

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ttacon/chalk"
)

type Device struct {
	UUID        string `json:"uuid"`
	Group       string `json:"group"`
	Name        string
	Brand       string `json:"brand"`
	Number      int    `json:"number"`
	Memory      int    `json:"memory"`
	Utilization int    `json:"utilization"`
	Temperature int    `json:"temperature"`
}
type ClaimInfo struct {
	User     string `json:"user"`
	Duration int64  `json:"duration"`
}

type ProcessInfo struct {
	PID      int    `json:"pid"`
	Command  string `json:"command"`
	Owner    string `json:"owner"`
	Context  string `json:"context"`
	Duration int64  `json:"duration"`
}

type DeviceStatus struct {
	Info      Device        `json:"info"`
	Claim     ClaimInfo     `json:"claim"`
	Processes []ProcessInfo `json:"processes"`
}

type NodeStatus struct {
	Devices map[string]DeviceStatus `json:"devices"`
}

type ClaimStatus struct {
	Devices []Device `json:"devices"`
}

func request(serverName string, resourceCount int, timeout int) ([]int, error) {

	var err error = nil
	var response *http.Response

	if timeout > 0 {

		client := http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}

		response, err = client.Get(fmt.Sprintf("http://%s/wait?gpu=%d", serverName, resourceCount))

	} else {

		response, err = http.Get(fmt.Sprintf("http://%s/wait?gpu=%d", serverName, resourceCount))

	}

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	var result ClaimStatus

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(response.Body)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body.Bytes(), &result)

	if err != nil {

		return nil, err

	}

	var ids []int

	for _, d := range result.Devices {
		ids = append(ids, d.Number)
	}

	return ids, nil

}

func status(server string) error {

	var err error = nil
	var response *http.Response

	surl, err := url.Parse(server)

	if err != nil {
		return err
	}

	if strings.Compare(surl.Scheme, "unix") == 0 {

		client := http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", surl.Path)
				},
			},
		}

		response, err = client.Get("http://cogs/")

	} else {

		client := http.Client{}

		response, err = client.Get(server)

	}

	if err != nil {
		return err
	}

	defer response.Body.Close()

	var status NodeStatus

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(response.Body)

	if err != nil {
		return err
	}

	err = json.Unmarshal(body.Bytes(), &status)

	if err != nil {

		return err

	}

	devices := make([]DeviceStatus, len(status.Devices))
	i := 0

	for _, d := range status.Devices {
		devices[i] = d
		i++
	}

	sort.SliceStable(devices, func(i, j int) bool {
		return devices[i].Info.Number < devices[j].Info.Number
	})

	for _, device := range devices {

		var lineStyle, utilizationStyle chalk.Color

		if device.Info.Utilization == 0 {
			lineStyle = chalk.Green
		} else if device.Claim.User != "" {
			lineStyle = chalk.Red
		} else {
			lineStyle = chalk.Yellow
		}

		if device.Info.Utilization > 0 {
			utilizationStyle = chalk.Red
		} else {
			utilizationStyle = chalk.ResetColor
		}

		fmt.Print(chalk.ResetColor)

		fmt.Printf("%s%-10d", lineStyle, device.Info.Number)
		fmt.Printf("|  %s%10d%%", utilizationStyle, device.Info.Utilization)
		fmt.Printf("|  %s%10d%%", lineStyle, device.Info.Memory)
		fmt.Printf("|  %s (%d)\n", device.Claim.User, device.Claim.Duration)

		for _, process := range device.Processes {

			var processStyle chalk.Color

			if process.Owner == device.Claim.User {
				processStyle = chalk.Green
			} else {
				processStyle = chalk.Red
			}

			fmt.Printf("%s * %d: %s [%s]\n", processStyle, process.PID, process.Owner, process.Command)
		}

	}

	return nil

}

func main() {

	var resourceCount = flag.Int("n", 0, "How many GPUs to claim")
	var server = flag.String("s", "unix:///var/run/cogs.sock", "CoGS server address")
	var timeout = flag.Int("t", 1, "Wait for GPUs for duration in seconds (negative value means indefinetely)")

	flag.Parse()

	if flag.Parsed() {

		command := flag.Args()

		if *resourceCount <= 0 && len(command) == 0 {

			err := status(*server)

			if err != nil {
				panic(err)
			}

		} else {

			ids, err := request(*server, *resourceCount, *timeout)
			if err != nil {
				panic(err)
			}

			cuda_ids := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(ids)), ","), "[]")

			if len(command) > 0 {

				cuda_env := fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", cuda_ids)

				err = syscall.Exec(command[0], command[1:], []string{cuda_env})

				if err != nil {
					panic(err)
				}

			} else {

				fmt.Println(cuda_ids)
				os.Exit(0)

			}

		}

	} else {
		flag.PrintDefaults()
		os.Exit(1)
	}

}
