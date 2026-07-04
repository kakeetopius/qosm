//go:build ignore

package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const fmtString = `
package service

// Created by genservices, don't edit manually
// Generated at %s
// Fetched from %q

// TCPPortNames contains the port names for all TCP ports.
var TCPPortNames = tcpPortNames

// UDPPortNames contains the port names for all UDP ports.
var UDPPortNames = udpPortNames

var tcpPortNames = map[TCPPort]string{
%s}
var udpPortNames = map[UDPPort]string{
%s}
`

var url = flag.String("url", "http://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xml", "URL to grab port numbers from")

func main() {
	fmt.Fprintf(os.Stderr, "Fetching ports from %q\n", *url)
	resp, err := http.Get(*url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "Parsing XML")
	var registry struct {
		Records []struct {
			Protocol string `xml:"protocol"`
			Number   string `xml:"number"`
			Name     string `xml:"name"`
		} `xml:"record"`
	}
	xml.Unmarshal(body, &registry)
	var tcpPorts bytes.Buffer
	var udpPorts bytes.Buffer
	done := map[string]map[int]bool{
		"tcp": {},
		"udp": {},
	}
	for _, r := range registry.Records {
		port, convErr := strconv.Atoi(r.Number)
		if convErr != nil {
			continue
		}
		if r.Name == "" {
			continue
		}
		var b *bytes.Buffer
		switch r.Protocol {
		case "tcp":
			b = &tcpPorts
		case "udp":
			b = &udpPorts
		default:
			continue
		}
		if done[r.Protocol][port] {
			continue
		}
		done[r.Protocol][port] = true
		fmt.Fprintf(b, "\t%d: %q,\n", port, r.Name)
	}
	servicesFile, err := os.OpenFile("./internal/service/iana_ports.go", os.O_TRUNC|os.O_RDWR, 0o754)
	if err != nil {
		panic(err)
	}
	defer servicesFile.Close()
	fmt.Fprintf(servicesFile, fmtString, time.Now(), *url, tcpPorts.String(), udpPorts.String())
	fmt.Println("DONE")
}
