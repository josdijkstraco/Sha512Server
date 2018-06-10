package main

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// global variables
var done chan bool              // channel to signal the main thread that the process can end
var mapLock sync.RWMutex        // read/write mutex to the map that contains the sha512 hashes
var shaMap map[uint64]string    // container that maps sequence number to sha512 hash
var elapsedTime time.Duration   // variable to keep track of total processing time
var globalSequenceNumber uint64 // variable to keep track of sequence numbers
var processedCount uint64       // variable to keep track of sha512 calculations count
var srv *http.Server            // reference to the http server
var shuttingdown bool

// JSONStatistics is the struct that stores statistical information
type JSONStatistics struct {
	Total   int `json:"total"`
	Average int `json:"average"`
}

// This method is called by the http server when a request is received
// for the /hash/ endpoint, which is used for requesting sha512 hashes
func handleGetHashRequest(writer http.ResponseWriter, request *http.Request) {
	// only accept GET requests
	if request.Method != "GET" {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// continue with the GET request
	// the requested path should consist of a single positive number only
	var sequenceNumber = uint64(0)
	var remainder string

	// parse the sequence number out of the request,
	// also make sure the request is properly formatted
	fmt.Sscanf(request.URL.Path[1:], "hash/%d%s", &sequenceNumber, &remainder)
	if remainder != "" || sequenceNumber == 0 {
		http.NotFound(writer, request)
		return
	}

	// handle the requested sequence number by using a Read lock
	mapLock.RLock()
	hash, ok := shaMap[sequenceNumber]
	mapLock.RUnlock()

	// if the key was found in the map, return the hash
	if ok == true {
		fmt.Fprint(writer, hash)
	} else {
		http.NotFound(writer, request)
	}
}

// Function that handles the request to calculate a new sha512 hash value
// for the specified password.
func handlePostHashRequest(writer http.ResponseWriter, request *http.Request) {
	// check if this is a POST request
	if request.Method != "POST" {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// find the password parameter
	value := request.FormValue("password")
	if value == "" {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// get a new unique sequence number for this request
	sequenceNumber := atomic.AddUint64(&globalSequenceNumber, 1)

	// send the sequence number to the client
	fmt.Fprintf(writer, "%d", sequenceNumber)

	// this function needs to return in order to send the response,
	// so kick off a new thread to continue processing the password
	go processHashRequest(value, sequenceNumber)
}

// This function calculates the hash and stores it in the map.
func processHashRequest(value string, sequenceNumber uint64) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Caught panic: %v", err)
		}
	}()

	// wait 5 seconds
	time.Sleep(5 * time.Second)

	// calculate hash, and measure how long it takes to calculate it
	start := time.Now()
	hasher := sha512.New()
	hasher.Write([]byte(value))
	hash := hasher.Sum(nil)
	b64Password := base64.StdEncoding.EncodeToString(hash)
	elapsed := time.Since(start)

	// aquire Write lock
	mapLock.Lock()
	defer mapLock.Unlock()

	shaMap[sequenceNumber] = string(b64Password)
	elapsedTime += elapsed
	processedCount++

	// I still have the lock, might as well check if I'm
	// the last one to finish before shutting down
	if shuttingdown == true {
		verifyShutdownStatus()
	}
}

// This function handles requests on the /stat endpoint.
func handleStatisticsRequest(writer http.ResponseWriter, request *http.Request) {
	fmt.Println(request.RequestURI)

	if request.Method != "GET" {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	statistics := JSONStatistics{}

	statistics.Total = int(globalSequenceNumber)
	if processedCount == 0 {
		statistics.Average = 0
	} else {
		statistics.Average = int(float64(elapsedTime.Nanoseconds()) / float64(processedCount))
	}

	// encode as json
	buffer, err := json.Marshal(statistics)
	if err == nil {
		fmt.Fprint(writer, string(buffer))
	} else {
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// This function handles requests received on the /shutdown endpoint.
func handleShutdownRequest(writer http.ResponseWriter, request *http.Request) {
	// reply to the request
	http.Error(writer, http.StatusText(http.StatusOK), http.StatusOK)

	// let this function return so the response can be sent, and continue with the shutdown checks
	go processShutdown()
}

// This function is called when a shutdown request is received, and checks whether
// a shutdown is pending, and if not, initiate the shutdown process.
func processShutdown() {
	// use the (Write) lock used for shaMap, since the handleHash functions
	// use this as well to check for pending shutdown
	mapLock.Lock()
	defer mapLock.Unlock()

	if shuttingdown == false {
		shuttingdown = true
		srv.Close() // ignoring ptential error status of Close, what can you do about it?
		verifyShutdownStatus()
	}
}

// This function checks if all requests have been processed.
// Only call this function with a lock on mapLock.
func verifyShutdownStatus() {
	if shuttingdown == true && processedCount == atomic.LoadUint64(&globalSequenceNumber) {
		done <- true
	}
}

// HandleFunction defines the interface of the wrapped http handle functions
type HandleFunction func(http.ResponseWriter, *http.Request)

// This is a helper function to wrap the http handle functions in, for error
// catching and robustness purposes.
func logIssues(function HandleFunction) HandleFunction {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Caught panic: %v", err)
			}
		}()
		function(writer, request)
	}
}

// This function initializes the global variables.
func initGlobalVariables() {
	done = make(chan bool)
	mapLock = sync.RWMutex{}
	globalSequenceNumber = 0
	processedCount = 0
	shaMap = make(map[uint64]string)
	shuttingdown = false
}

// This function sets up the endpoint handlers.
func setupEndpointHandlers() {
	http.HandleFunc("/hash", logIssues(handlePostHashRequest))     // for POST requests with a password
	http.HandleFunc("/hash/", logIssues(handleGetHashRequest))     // for GET requests with a sequence number
	http.HandleFunc("/shutdown", logIssues(handleShutdownRequest)) // for requests to shutdown the service
	http.HandleFunc("/stats", logIssues(handleStatisticsRequest))  // for GET requests to receive statistics
}

func main() {
	// initialize global variables
	initGlobalVariables()

	// setup the endpoints we have to respond to
	setupEndpointHandlers()

	// start the server
	srv = &http.Server{Addr: "localhost:8080"}
	err := srv.ListenAndServe()

	// The ListenAndServe function exists either due to an error starting the http listener,
	// or when the listener is forcefully closed due to a shutdown request. Distinhuish between these two.
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "server closed") == false {
			log.Printf("Error starting the server: %s", err.Error())
		} else {
			// this is when the server was stopped on purpose, so wait for the message from the shutdown thread
			<-done
		}
	}
}
