// Simple example app that counts requests and reports stats to InfluxDB/Kapacitor
package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// Requests counter
var requests int64

func main() {
	host, _ := os.Hostname()
	replicaset := os.Getenv("APP_REPLICASET")
	url := os.Getenv("APP_INFLUXDB_URL")
	log.Printf("host: %s, replicaset: %s, url: %s", host, replicaset, url)
	go stats(host, replicaset, url)
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8000", nil)
}

// handler responds to an HTTP request with the current number of requests this process has served
func handler(w http.ResponseWriter, r *http.Request) {
	for {
		r := atomic.LoadInt64(&requests)
		if atomic.CompareAndSwapInt64(&requests, r, r+1) {
			fmt.Fprintf(w, "Current request count %d\n", r)
			break
		}
	}
}

// stats reports the current request count to a given URL in InfluxDB line protocol, every second.
func stats(host, replicaset, url string) {
	// construct a line protocol line for this host
	line := fmt.Sprintf("requests,host=%s,replicaset=%s value=", host, replicaset)

	// create buffer for writing line protocol data
	var buf bytes.Buffer

	ticker := time.Tick(time.Second)
	for range ticker {
		r := atomic.LoadInt64(&requests)
		buf.Reset()
		// Write line
		buf.WriteString(line)
		// Write current request value
		buf.WriteString(strconv.FormatInt(r, 10))
		buf.WriteString("i\n")

		// post line protocol data to URL
		resp, err := http.Post(url, "text/plain", &buf)
		if err != nil {
			log.Println(err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			log.Printf("Unexpected response code %d", resp.StatusCode)
		}
	}
}
