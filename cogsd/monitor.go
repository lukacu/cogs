package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type SMI struct {
	XMLName xml.Name      `xml:"nvidia_smi_log"`
	Devices []Description `xml:"gpu"`
}

type Description struct {
	XMLName xml.Name `xml:"gpu"`
	Name    string   `xml:"product_name"`
	Brand   string   `xml:"product_brand"`
	UUID    string   `xml:"uuid"`
	Number  int      `xml:"minor_number"`
}

type Monitor struct {
	SMIExecutable string `default:"nvidia-smi" yaml:"smi"`
	Devices       []Device
	Mutex         sync.RWMutex
}

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

type Claim struct {
	DeviceNumber int
	PID          int
}

func split_tokens(c rune) bool {
	return c == ' ' || c == '\t'
}

func parse_int(s string) (int, error) {
	if strings.Compare(s, "-") == 0 {
		return 0, nil
	} else {
		i, err := strconv.Atoi(s)

		if err != nil {
			return 0, err
		} else {
			return i, nil
		}
	}
}

func parse_ints(tokens []string) ([]int, error) {

	var conv = []int{}

	for _, i := range tokens {
		j, err := parse_int(i)
		if err != nil {
			return conv, err
		}
		conv = append(conv, j)
	}

	return conv, nil
}

func (m *Monitor) dmon() error {
	cmd := exec.Command(m.SMIExecutable, "dmon")
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(r)

	go func() {

		defer log.Printf("Stopping device monitor")

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "#") {
				continue
			}

			tokens := strings.FieldsFunc(line, split_tokens)

			if len(tokens) != 10 {
				continue
			}

			values, err := parse_ints(tokens)

			if err != nil {
				log.Panicf("%s", err)

				continue
			}

			m.Mutex.Lock()

			device, err := m.find(values[0])

			if err != nil {
				log.Panicf("%s", err)
				m.Mutex.Unlock()
				continue
			}

			device.Memory = values[5]
			device.Utilization = values[4]
			device.Temperature = values[2]

			m.Mutex.Unlock()

			bus.Publish("dmon:update", *device)

		}

	}()

	// Start the command and check for errors
	err := cmd.Start()

	if err != nil {
		log.Printf("Starting device monitor")
	}

	return err
}

func (m *Monitor) pmon() error {
	cmd := exec.Command(m.SMIExecutable, "pmon")
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(r)

	go func() {

		defer log.Printf("Stopping process monitor")

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "#") {
				continue
			}

			tokens := strings.FieldsFunc(line, split_tokens)

			if len(tokens) != 8 {
				continue
			}

			id, err := parse_int(tokens[0])

			if err != nil {
				continue
			}

			pid, err := parse_int(tokens[1])

			if err != nil {
				continue
			}

			_, err = m.find(id)

			if err != nil {
				continue
			}

			bus.Publish("pmon:claim", Claim{DeviceNumber: id, PID: pid})

		}

	}()

	// Start the command and check for errors
	err := cmd.Start()

	if err == nil {
		log.Printf("Starting process monitor")
	}

	return err
}

func (m *Monitor) start() error {

	out, err := exec.Command(m.SMIExecutable, "-q", "-x").Output()

	if err != nil {
		return err
	}

	var smidata SMI
	if err = xml.Unmarshal(out, &smidata); err == nil {

		m.Devices = make([]Device, len(smidata.Devices))

		for i, d := range smidata.Devices {

			m.Devices[i] = Device{UUID: d.UUID, Number: d.Number, Brand: d.Brand, Memory: 0}

		}

	} else {
		return err
	}

	if err = m.dmon(); err != nil {
		return err
	}

	if err = m.pmon(); err != nil {
		return err
	}

	return nil

}

func (m *Monitor) find(i int) (*Device, error) {

	for _, d := range m.Devices {

		if d.Number == i {
			return &d, nil
		}

	}

	return nil, errors.New("Device does not exist")

}
