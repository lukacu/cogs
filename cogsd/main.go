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
	"strconv"
	"sync"

	"github.com/asaskevich/EventBus"
	"github.com/creasty/defaults"
)

var (
	listenUDS string = "/var/run/cogs.sock"
	listenTCP string = ":9000"
	binarySMI string = "nvidia-smi"
	docker    string = "/var/run/docker.sock"

	bus EventBus.Bus = EventBus.New()
)

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
		fmt.Println("Unable to start CUDA monitor: %s", err)
		os.Exit(-1)
	}

	waiter := new(sync.WaitGroup)
	waiter.Add(2)

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveRoot)
	mux.HandleFunc("/api", serveAPI)

	if listenUDS != "" {
		listener, err := net.Listen("unix", listenUDS)

		if err == nil {
			fmt.Println("Unable to open UDS socket on %s: %s", listenUDS, err)
			os.Exit(-1)
		}

		go func() {
			http.Serve(listener, mux)
			waiter.Done()
		}()

	} else {
		waiter.Done()
	}

	if listenTCP != "" {
		listener, err := net.Listen("tcp", listenTCP)

		if err == nil {
			fmt.Println("Unable to open TCP socket on %s: %s", listenUDS, err)
			os.Exit(-1)
		}

		go func() {
			http.Serve(listener, mux)
			waiter.Done()
		}()

	} else {
		waiter.Done()
	}

	waiter.Wait()
}
