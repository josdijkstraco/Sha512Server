package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestValidRequest(t *testing.T) {
	// start the server process
	x := exec.Command("./../Server/server.exe")
	starterr := x.Start()
	if starterr != nil {
		t.Errorf("Could not start server executable")
		return
	}

	// send a hash request
	post, posterr := http.PostForm("http://localhost:8080/hash", url.Values{"password": {"angryMonkey"}})
	if posterr != nil {
		t.Errorf("Could not connect to the server")
	}
	postrsp, _ := ioutil.ReadAll(post.Body)
	if string(postrsp) != "1" {
		t.Errorf("Not responding with '1'")
	}
	post.Body.Close()

	resp, _ := http.Get("http://localhost:8080/hash/1")
	body, _ := ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "404 page not found") == false {
		t.Errorf("Not responding with 404")
	}
	resp.Body.Close()

	resp, _ = http.Get("http://localhost:8080/stats")
	body, _ = ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "{\"total\":1,\"average\":0}") == false {
		t.Errorf("Wrong stats response")
	}
	resp.Body.Close()

	time.Sleep(6 * time.Second)

	resp, _ = http.Get("http://localhost:8080/hash/1")
	body, _ = ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q==") == false {
		t.Errorf("Wrong hash returned")
	}
	resp.Body.Close()

	resp, _ = http.Get("http://localhost:8080/stats")
	body, _ = ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "{\"total\":1,\"average\":") == false {
		t.Errorf("Wrong stats response")
	}
	resp.Body.Close()
}

func TestShutdown(t *testing.T) {
	// start the server process
	x := exec.Command("./../Server/server.exe")
	x.Start()

	// send a hash request
	_, posterr := http.PostForm("http://localhost:8080/hash", url.Values{"password": {"angryMonkey"}})
	if posterr != nil {
		t.Fail()
	}

	// send a shutdown request
	shutdown, shutdownerr := http.Get("http://localhost:8080/shutdown")
	if shutdownerr != nil {
		t.Errorf("Server did not accept shutdown request: %s", shutdownerr.Error())
	}
	body, _ := ioutil.ReadAll(shutdown.Body)
	if strings.Contains(string(body), "OK") == false {
		t.Errorf("Not responding with OK")
	}

	// wait 1 second
	time.Sleep(1 * time.Second)

	// since the hash request is pending, the process should be running
	if checkAcceptingConnections() == true {
		t.Errorf("Process is still accepting connections")
	}

	// wait 5 seconds
	time.Sleep(6 * time.Second)

	// process should have ended
	if checkAcceptingConnections() == true {
		t.Errorf("Process is still accepting connections")
	}
}

func TestManyRequests(t *testing.T) {
	// start the server process
	x := exec.Command("./../Server/server.exe")
	x.Start()

	time.Sleep(1 * time.Second)

	for i := 0; i < 1000; i++ {
		SendRequest(t)
	}

	// for simplicity, just sleep
	time.Sleep(10 * time.Second)

	// send a shutdown request: check if it is still running
	_, shutdownerr := http.Get("http://localhost:8080/shutdown")
	if shutdownerr != nil {
		t.Errorf("Server did not accept shutdown request: %s", shutdownerr.Error())
	}
}

func TestInvalidRequests(t *testing.T) {
	// start the server process
	x := exec.Command("./../Server/server.exe")
	x.Start()

	time.Sleep(1 * time.Second)

	post, _ := http.PostForm("http://localhost:8080/has", url.Values{"password": {"angryMonkey"}})
	body, _ := ioutil.ReadAll(post.Body)
	if (strings.Contains(string(body), "404 page not found")) == false {
		t.Errorf("Server accepted /has endpoint")
	}
	post.Body.Close()

	post, _ = http.PostForm("http://localhost:8080/hash/", url.Values{"passwqord": {"angryMonkey"}})
	body, _ = ioutil.ReadAll(post.Body)
	if (strings.Contains(string(body), "Bad Request")) == false {
		t.Errorf("Server accepted wrong parameter")
	}
	post.Body.Close()

	post, _ = http.Get("http://localhost:8080/hash/1j")
	body, _ = ioutil.ReadAll(post.Body)
	if (strings.Contains(string(body), "404 page not found")) == false {
		t.Errorf("Server accepted 1j endpoint")
	}
	post.Body.Close()

	post, _ = http.Get("http://localhost:8080/hash/1/1")
	body, _ = ioutil.ReadAll(post.Body)
	if (strings.Contains(string(body), "404 page not found")) == false {
		t.Errorf("Server accepted 1/1 endpoint")
	}
	post.Body.Close()

	_, _ = http.Get("http://localhost:8080/shutdown")
}

// send a message to check if the process is running since the os functions
// return valid data for a process based on PID if even the process is not running
func checkAcceptingConnections() bool {
	_, err := http.Get("http://localhost:8080/stats")
	return err == nil
}

func SendRequest(t *testing.T) {
	// send a hash request
	post, posterr := http.PostForm("http://localhost:8080/hash", url.Values{"password": {"angryMonkey"}})
	if posterr != nil {
		t.Fail()
	}
	_, _ = ioutil.ReadAll(post.Body)
	post.Body.Close()
}
