package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc/jsonrpc"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/asaskevich/EventBus"
	"github.com/creasty/defaults"
)

var (
	listenUDS string = "/var/run/cogs.sock"
	listenTCP string = ":9110"
	docker    string = "/var/run/docker.sock"

	bus EventBus.Bus = EventBus.New()
)

func handleInterrupt() chan os.Signal {
	sig := make(chan os.Signal, 1)

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	return sig
}

type ClaimInfo struct {
	User     string `json:"user"`
	Duration int64  `json:"duration"`
}

type ProcessInfo struct {
	PID      int64  `json:"pid"`
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
	Devices map[int]DeviceStatus `json:"devices"`
}

var status NodeStatus = NodeStatus{Devices: make(map[int]DeviceStatus)}

func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func lookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("LookupEnvOrInt[%s]: %v", key, err)
		}
		return v
	}
	return defaultVal
}

func serveRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func serveAPI(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		http.Error(w, "method must be connect", 405)
		return
	}
	//req.RemoteAddr
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer conn.Close()
	io.WriteString(conn, "HTTP/1.0 Connected\r\n\r\n")
	jsonrpc.ServeConn(conn)
}

func OnClaim(c Claim) {
	if c.PID == 0 {
		log.Printf("Claim %d: none ", c.DeviceNumber)
		return
	}

	user, err := identifyProcess(c.PID)

	if err != nil {
		log.Printf("Unable to determine owner of process %d: %s", c.PID, err)
		return
	}

	log.Printf("Claim %d: %s (pid: %d) ", c.DeviceNumber, user, c.PID)

}

func OnDeviceStatus(d Device) {

	ds, ok := status.Devices[d.Number]

	if ok {
		ds.Info = d
		status.Devices[d.Number] = ds
	} else {
		status.Devices[d.Number] = DeviceStatus{Info: d, Claim: ClaimInfo{}}
	}

}

func main() {

	flag.StringVar(&listenUDS, "listen-uds", lookupEnvOrString("COGS_UDS_SOCKET", listenUDS), "Listen on Unix Domain socket")
	flag.StringVar(&listenTCP, "listen-tcp", lookupEnvOrString("COGS_TCP_SOCKET", listenTCP), "Listen on TCP socket")
	flag.StringVar(&docker, "docker", lookupEnvOrString("DOCKER_SOCKET", docker), "Connect to docker API socket")
	//flag.IntVar(&HTTP_Timeout, "http-timeout", LookupEnvOrInt("HTTP_TIMEOUT", HTTP_Timeout), "http timeout requesting http services")

	flag.Parse()

	monitor := &Monitor{}

	if err := defaults.Set(monitor); err != nil {
		panic(err)
	}

	err := monitor.start()

	if err != nil {
		log.Fatalf("Unable to start CUDA monitor: %s", err)
		os.Exit(-1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveRoot)
	mux.HandleFunc("/api", serveAPI)

	bus.Subscribe("pmon:claim", OnClaim)
	bus.Subscribe("dmon:update", OnDeviceStatus)

	var udsSocket net.Listener = nil
	var tcpSocket net.Listener = nil

	if listenUDS != "" {
		udsSocket, err = net.Listen("unix", listenUDS)

		if err != nil {
			log.Fatalf("Unable to open UDS socket on %s: %s", listenUDS, err)
			os.Exit(-1)
		}

		go func() {
			http.Serve(udsSocket, mux)
		}()

	}

	if listenTCP != "" {
		tcpSocket, err = net.Listen("tcp", listenTCP)

		if err != nil {
			log.Fatalf("Unable to open TCP socket on %s: %s", listenTCP, err)
			os.Exit(-1)
		}

		go func() {
			http.Serve(tcpSocket, mux)
		}()
	}

	<-handleInterrupt()

	log.Print("Shutting down")

	if listenUDS != "" {
		udsSocket.Close()
	}

	if listenTCP != "" {
		tcpSocket.Close()
	}

	os.Exit(0)

}
