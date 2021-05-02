package http

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
)

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Proxy is an object to manage HTTP Requests
type Proxy struct {
	RegisterdServices *RegisteredServicesAndPorts
}

//RegisteredServicesAndPorts is a Map for storing service to port relationship
type RegisteredServicesAndPorts = struct {
	sync.RWMutex
	ServiceToPortMap map[string]string
}

func (p *Proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {

	client := &http.Client{}

	req.RequestURI = ""

	p.RegisterdServices.RLock()
	targetHost := p.RegisterdServices.ServiceToPortMap[req.Host]
	p.RegisterdServices.RUnlock()

	req.URL, _ = url.Parse(fmt.Sprintf("http://%s%s", targetHost, req.URL))
	log.Println("Selected remote endpoint:", targetHost, "for Host:", req.Host)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Fatal("ServeHTTP:", err)
	}
	defer resp.Body.Close()

	log.Println("Response: ", req.RemoteAddr, " ", resp.Status)

	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
}
