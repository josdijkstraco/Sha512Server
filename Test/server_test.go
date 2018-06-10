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
	post, posterr := http.PostForm("http://localhost:8100/hash", url.Values{"password": {"angryMonkey"}})
	if posterr != nil {
		t.Errorf("Could not connect to the server")
	}
	postrsp, _ := ioutil.ReadAll(post.Body)
	if string(postrsp) != "1" {
		t.Errorf("Not responding with '1'")
	}
	post.Body.Close()

	resp, _ := http.Get("http://localhost:8100/hash/1")
	body, _ := ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "404 page not found") == false {
		t.Errorf("Not responding with 404")
	}
	resp.Body.Close()

	resp, _ = http.Get("http://localhost:8100/stats")
	body, _ = ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "{\"total\":1,\"average\":0}") == false {
		t.Errorf("Wrong stats response")
	}
	resp.Body.Close()

	time.Sleep(6 * time.Second)

	resp, _ = http.Get("http://localhost:8100/hash/1")
	body, _ = ioutil.ReadAll(resp.Body)
	if strings.Contains(string(body), "ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q==") == false {
		t.Errorf("Wrong hash returned")
	}
	resp.Body.Close()

	resp, _ = http.Get("http://localhost:8100/stats")
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
	_, posterr := http.PostForm("http://localhost:8100/hash", url.Values{"password": {"angryMonkey"}})
	if posterr != nil {
		t.Fail()
	}

	// send a shutdown request
	_, shutdownerr := http.Get("http://localhost:8100/shutdown")
	if shutdownerr != nil {
		t.Errorf("Server did not accept shutdown request: %s", shutdownerr.Error())
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
	//pid := x.Process.Pid//

	time.Sleep(5 * time.Second)

	for i := 0; i < 100; i++ {
		SendRequest(t)
	}

	// for simplicity, just sleep
	time.Sleep(10 * time.Second)

	// send a shutdown request
	_, shutdownerr := http.Get("http://localhost:8100/shutdown")
	if shutdownerr != nil {
		t.Errorf("Server did not accept shutdown request: %s", shutdownerr.Error())
	}
}

// send a message to check if the process is running since the os functions
// return valid data for a process based on PID if even the process is not running
func checkAcceptingConnections() bool {
	_, err := http.Get("http://localhost:8100/stats")
	return err == nil
}

func SendRequest(t *testing.T) {
	// send a hash request
	post, posterr := http.PostForm("http://localhost:8100/hash", url.Values{"password": {"angryMonkey"}})
	if posterr != nil {
		t.Fail()
	}
	_, _ = ioutil.ReadAll(post.Body)
	post.Body.Close()
}
