package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

type Device struct {
	Info DeviceInfo `json:"info"`
}

type DeviceInfo struct {
	Number int `json:"number"`
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

	var devices []DeviceInfo

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(response.Body)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body.Bytes(), &devices)

	if err != nil {

		return nil, err

	}

	var ids []int

	for _, d := range devices {
		ids = append(ids, d.Number)
	}

	return ids, nil

}

func main() {

	var resourceCount = flag.Int("n", 1, "How many GPU resources")
	var serverName = flag.String("s", "claims", "Resource claims server")
	var timeout = flag.Int("t", 1, "Wait for resources for duration in seconds")

	flag.Parse()

	if flag.Parsed() {

		ids, err := request(*serverName, *resourceCount, *timeout)
		if err != nil {
			panic(err)
		}

		command := flag.Args()

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

	} else {
		flag.PrintDefaults()
		os.Exit(1)
	}

}
