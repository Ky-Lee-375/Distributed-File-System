package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"cs425/queue"
	"cs425/server"
	"cs425/timer"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const JOIN_RETRY_TIMEOUT = time.Second * 10
const JOIN_OK = "JOIN_OK"
const JOIN_ERROR = "JOIN_ERROR"
const BAD_REQUEST = "BAD_REQUEST"
const JOIN_TIMER_ID = "JOIN_TIMER"
const DEFAULT_PORT = 6000
const NODES_PER_ROUND = 4 // Number of random peers to send gossip every round
const ERROR_ILLEGAL_REQUEST = JOIN_ERROR + "\n" + "Illegal Request" + "\n"

type Node struct {
	Hostname string
	Port     int
}

var cluster = []Node{
	{"fa23-cs425-7301.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7302.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7303.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7304.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7305.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7306.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7307.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7308.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7309.cs.illinois.edu", DEFAULT_PORT},
	{"fa23-cs425-7310.cs.illinois.edu", DEFAULT_PORT},
}

var local_cluster = []Node{
	{"localhost", 6001},
	{"localhost", 6002},
	{"localhost", 6003},
	{"localhost", 6004},
	{"localhost", 6005},
}

// Starts a UDP server on specified port
func main() {
	var err error
	var port int
	var hostname string
	var level string
	var env string
	var withSuspicion bool

	systemHostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	flag.IntVar(&port, "p", DEFAULT_PORT, "server port number")
	flag.StringVar(&hostname, "h", systemHostname, "server hostname")
	flag.StringVar(&level, "l", "DEBUG", "log level")
	flag.StringVar(&env, "e", "production", "environment: development, production")
	flag.BoolVar(&withSuspicion, "s", false, "gossip with suspicion")
	flag.Parse()

	if hostname == "localhost" || hostname == "127.0.0.1" {
		env = "development"
	}

	if env == "development" {
		cluster = local_cluster
		hostname = "localhost"
		log.Info("Using local cluster")
	}

	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	log.SetReportCaller(false)
	log.SetOutput(os.Stderr)

	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	}

	log.Debug(cluster)

	s, err := server.NewServer(hostname, port)
	if err != nil {
		log.Fatal(err)
	}

	if withSuspicion {
		s.Protocol = server.GOSPSIP_SUSPICION_PROTOCOL
	}

	if IsIntroducer(s) {
		s.Active = true
		log.Info("Introducer is online...")
	}

	// log.SetLevel(log.DebugLevel)
	// if os.Getenv("DEBUG") != "TRUE" {
	// 	log.SetLevel(log.InfoLevel)
	// 	logfile := fmt.Sprintf("%s.log", s.Self.Signature)
	// 	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	defer f.Close()
	// 	log.SetOutput(f)
	// 	os.Stderr.WriteString(fmt.Sprintf("Log File: %s\n", logfile))
	// }

	log.Infof("Server %s listening on port %d\n", s.Self.Signature, port)
	defer s.Close()
	startNode(s)
}

// Fix the first node as the introducer
func IsIntroducer(s *server.Server) bool {
	return s.Self.Hostname == cluster[0].Hostname && s.Self.Port == cluster[0].Port
}

// Start the node process and launch all the threads
func startNode(s *server.Server) {
	log.Infof("Node %s is starting...\n", s.Self.ID)

	go receiverRoutine(s)          // to receive requests from network
	go senderRoutine(s)            // to send gossip messages
	go inputRoutine(s)             // to receive requests from stdin
	go listenForClientReq(s)       // to receive operations requests from clients
	go receiveRequestFromLeader(s) // to process requests from master
	go queueDriver(s.RequestQueue) // to process the requests in the queue

	sendJoinRequest(s)

	// Blocks until either new message received or timer sends a signal
	for {
		select {
		case e := <-s.TimerManager.TimeoutChannel:
			HandleTimeout(s, e)
		case e := <-s.ReceiverChannel:
			HandleRequest(s, e)
		case e := <-s.InputChannel:
			HandleCommand(s, e)
		}
	}
}

// Sends membership list to random subset of peers every T_gossip period
// Updates own counter and timestamp before sending the membership list
func sendPings(s *server.Server) {
	targets := selectRandomTargets(s, NODES_PER_ROUND)
	if len(targets) == 0 {
		return
	}

	log.Debugf("Sending gossip to %d hosts", len(targets))

	s.MemberLock.Lock()
	s.Self.Counter++
	s.Self.UpdatedAt = time.Now().UnixMilli()
	s.MemberLock.Unlock()

	for _, target := range targets {
		message := s.GetPingMessage(target.ID)
		n, err := s.Connection.WriteToUDP([]byte(message), target.Address)
		if err != nil {
			log.Println(err)
			continue
		}
		s.TotalByte += n
		log.Debugf("Sent %d bytes to %s\n", n, target.Signature)
	}
}

// Selects at most 'count' number of hosts from list
func selectRandomTargets(s *server.Server, count int) []*server.Host {
	s.MemberLock.Lock()
	defer s.MemberLock.Unlock()

	var hosts = []*server.Host{}
	for _, host := range s.Members {
		if host.ID != s.Self.ID {
			hosts = append(hosts, host)
		}
	}

	// shuffle the array
	for i := len(hosts) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		hosts[i], hosts[j] = hosts[j], hosts[i]
	}

	if len(hosts) < count {
		return hosts
	}
	return hosts[:count]
}

// Timeout signal received from timer
// Either suspect node or mark failed
func HandleTimeout(s *server.Server, e timer.TimerEvent) {
	s.MemberLock.Lock()
	defer s.MemberLock.Unlock()

	// should never happen btw
	if e.ID == s.Self.ID {
		return
	}

	if e.ID == JOIN_TIMER_ID {
		if IsIntroducer(s) {
			if len(s.Members) <= 1 {
				log.Info("Timeout: Retrying JOIN.")
				sendJoinRequest(s)
			}
		} else if !s.Active {
			log.Info("Timeout: Retrying JOIN.")
			sendJoinRequest(s)
		}

		return
	}

	host, ok := s.Members[e.ID]

	// Ignore timeout for node which does not exist
	if !ok {
		return
	}

	// currentTime := time.Now()
	// // Format the current time as "hour:minute:second:millisecond"
	// timestamp := currentTime.Format("15:04:05:000")
	timestamp := time.Now().UnixMilli()

	if host.State == server.NODE_ALIVE {
		if s.Protocol == server.GOSSIP_PROTOCOL {
			log.Warnf("FAILURE DETECTED: (%d) Node %s is considered failed\n", timestamp, host.Signature)
			host.State = server.NODE_FAILED
		} else {
			log.Warnf("FAILURE SUSPECTED: (%d) Node %s is suspected of failure\n", timestamp, host.Signature)
			host.State = server.NODE_SUSPECTED
		}
		s.RestartTimer(e.ID, host.State)
	} else if host.State == server.NODE_SUSPECTED {
		log.Warnf("FAILURE DETECTED: (%d) Node %s is considered failed\n", timestamp, host.Signature)
		host.State = server.NODE_FAILED
		s.RestartTimer(e.ID, host.State)
	} else if host.State == server.NODE_FAILED {
		log.Warn("Deleting node from membership list...", host.Signature)
		delete(s.Members, e.ID)
	}

	s.MemberLock.Unlock()
	s.PrintMembershipTable()
	s.MemberLock.Lock()
}

// This routine gossips the membership list every GossipPeriod.
// It listens for start/stop signals on the GossipChannel.
func senderRoutine(s *server.Server) {
	active := true

	for {
		select {
		case active = <-s.GossipChannel:
			if active {
				log.Info("Starting gossip...")
			} else {
				log.Info("Stopping gossip...")
			}
		case <-time.After(server.T_GOSSIP):
			break
		}

		if active {
			// ghostEntryRemover(s)
			sendPings(s)
		}
	}
}

// This routine listens for messages to the server and forwards them to the ReceiverChannel.
func receiverRoutine(s *server.Server) {
	for {
		message, sender, err := s.GetPacket()
		if err != nil {
			log.Error(err)
			continue
		}
		s.ReceiverChannel <- server.ReceiverEvent{Message: message, Sender: sender}

	}
}

func sha1Hash(fileName string) string {
	hash := sha1.New()
	hash.Write([]byte(fileName))
	return hex.EncodeToString(hash.Sum(nil))
}

func getNodeID(hash string) int {
	// Find the right NodeID to store a file
	// Adding 1 to make the range 1-10
	nodeID := int(hash[0])%(len(cluster)) + 1
	return nodeID
}

func getAddressFromNodeID(nodeID int) (string, error) {
	index := nodeID - 1
	if index < 0 || index >= len(cluster) {
		return "", fmt.Errorf("invalid node ID")
	}
	return fmt.Sprintf("http://%s:%d/processRequest", cluster[index].Hostname, (nodeID + 6000 + 200)), nil
}

func listenForClientReq(s *server.Server) {
	log.Info("Starting HTTPS Server...")
	http.HandleFunc("/receiveRequest", func(w http.ResponseWriter, r *http.Request) {
		var request SDFSRequest
		// Decode the request body to SDFSRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, fmt.Sprintf("error decoding request: %v", err), http.StatusBadRequest)
			return
		}
		log.Printf("Received SDFSRequest message: %+v\n", request)

		fileNameHash := sha1Hash(request.LocalFilename)
		log.Printf("SHA-1 hash of filename: %s\n", fileNameHash)
		nodeIdToStore := getNodeID(fileNameHash)
		log.Printf("Found a nodeToStore: %d\n", nodeIdToStore)

		nodeAddress, err := getAddressFromNodeID(nodeIdToStore)
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}
		log.Printf("nodeAddress to send request %s\n", nodeAddress)

		// Convert the SDFSRequest into a JSON string
		jsonData, err := json.Marshal(request)
		if err != nil {
			log.Printf("Error converting SDFSRequest to JSON: %v\n", err)
			return
		}
		jsonString := string(jsonData)

		// Enqueue the nodeAddress and the JSON string representation of the request
		s.RequestQueue.Enqueue(nodeAddress, jsonString, int(request.Operation))

		s.RequestQueue.Print()

		// Send a response back
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request successfully enqueued"))

	})

	http.HandleFunc("/listenForFinalResponse", func(w http.ResponseWriter, r *http.Request) {
		var request SDFSRequest
		// Decode the request body to SDFSRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, fmt.Sprintf("error decoding request: %v", err), http.StatusBadRequest)
			return
		}
		log.Printf("Received Final Response: %+v\n", request)

		if request.RequesterVm != "" {
			destination := fmt.Sprintf("http://%s/processFinalResponse", request.RequesterVm)

			resp, err := http.Post(destination, "application/json", bytes.NewBuffer(request.Body))
			if err != nil {
				log.Println(err)
				return
			}
			defer resp.Body.Close()

			log.Println("Response Status of processFinalResponse : %s", resp.Status)
		} else {
			log.Println("No requester VM specified in the request.")
		}
	})

	port := s.Self.Port + 100 // this is just an example value, you can replace it with your desired port number
	address := fmt.Sprintf(":%s", strconv.Itoa(port))
	log.Printf("HTTP server is now listening on port :%d", port)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal("Error starting HTTP server:", err)
	}
}

func queueDriver(q *queue.Queue) {

	readCount := 0
	deferredReads := []*queue.QueueElement{}

	for {
		if !q.IsEmpty() {
			element, ok := q.Dequeue()
			if ok {
				// If the operation is a GET (read) and we have not reached the threshold
				if element.OpType == 1 && readCount < 4 {
					q.Dequeue() // Actually remove the element now
					sendHTTPRequest(element.Dest, element.RBody, element.OpType)
					readCount++
				} else if element.OpType == 1 && readCount >= 4 {
					deferredReads = append(deferredReads, &element) // Store the read request to process later
					q.Dequeue()
				} else { // Process PUT or DELETE (write operations)
					q.Dequeue()
					sendHTTPRequest(element.Dest, element.RBody, element.OpType)
					readCount = 0 // Reset the read count after processing a write

					// Process any deferred read requests
					for i, deferredRead := range deferredReads {
						sendHTTPRequest(deferredRead.Dest, deferredRead.RBody, deferredRead.OpType)
						deferredReads = append(deferredReads[:i], deferredReads[i+1:]...) // Remove the processed read from deferred reads
					}
				}
			}
		}

		// Add a short delay to avoid busy-waiting
		time.Sleep(100 * time.Millisecond)
	}
}

func sendHTTPRequest(destination string, requestBody string, OpType int) {
	url := destination
	body := strings.NewReader(requestBody)
	if OpType == 0 {
		method := "POST"
		log.Printf("Method type is set to %s \n", method)

		req, err := http.NewRequest(method, url, body)
		if err != nil {
			fmt.Println(err)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer resp.Body.Close()

		fmt.Println("Response Status POST:", resp.Status)

	} else if OpType == 1 {
		method := "GET"
		log.Printf("Method type is set to %s \n", method)

		req, err := http.NewRequest(method, url, body)
		if err != nil {
			fmt.Println(err)
			return
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer resp.Body.Close()

		fmt.Println("Response Status GET:", resp.Status)

	} else if OpType == 2 {
		method := "DELETE"
		log.Printf("Method type is set to %s \n", method)

	} else if OpType == 3 {
		method := "GET"
		log.Printf("Method type is set to %s \n", method)

	} else if OpType == 4 {
		method := "GET"
		log.Printf("Method type is set to %s \n", method)

	} else {
		log.Printf("Not supporting the operation type\n")

	}
}

func receiveRequestFromLeader(s *server.Server) {
	log.Info("Starting Request Receiver Server...")
	http.HandleFunc("/processRequest", func(w http.ResponseWriter, r *http.Request) {
		var request SDFSRequest
		// Decode the request body to SDFSRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, fmt.Sprintf("error decoding request: %v", err), http.StatusBadRequest)
			return
		}
		log.Printf("Received SDFSRequest message to process: %+v\n", request)

		// Handle Put
		if request.Operation == 0 {
			err := ioutil.WriteFile(request.SDFSFilename, request.Body, 0644)
			if err != nil {
				// Handle the error
				log.Fatalf("Failed to write to file %s: %v", request.SDFSFilename, err)
			}
		}

		// Handle Get
		if request.Operation == 1 {

		}

		// Handle Delete
		if request.Operation == 2 {

		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request successfully processed and added to SDFS"))
	})

	port := s.Self.Port + 200 // this is just an example value, you can replace it with your desired port number
	address := fmt.Sprintf(":%s", strconv.Itoa(port))
	log.Printf("Request processing server is now listening on port :%d", port)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal("Error starting receiver server:", err)
	}
}

func startGossip(s *server.Server) {
	if IsIntroducer(s) {
		log.Warn("Introducer is always active")
		return
	}

	if s.Active {
		log.Warn("server is already active")
		return
	}

	ID := s.SetUniqueID()
	log.Debugf("Updated Node ID to %s", ID)
	s.GossipChannel <- true
	sendJoinRequest(s)
}

func stopGossip(s *server.Server) {
	if IsIntroducer(s) {
		log.Warn("Introducer is always active")
		return
	}

	if !s.Active {
		log.Warn("server is already inactive")
		return
	}

	s.Active = false
	s.GossipChannel <- false
	s.MemberLock.Lock()
	for ID := range s.Members {
		delete(s.Members, ID)
	}
	s.MemberLock.Unlock()
	s.TimerManager.StopAll()
}

// Listen for commands on stdin
func inputRoutine(s *server.Server) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		s.InputChannel <- line
	}

	if err := scanner.Err(); err != nil {
		log.Warn("Error reading from stdin:", err)
	}
}

// Send request to join node and start timer
func sendJoinRequest(s *server.Server) {
	msg := s.GetJoinMessage()

	if IsIntroducer(s) {
		soleMember := true
		log.Printf("soleMember = %t", soleMember)
		for _, vm := range cluster {
			if vm.Hostname == s.Self.Hostname && vm.Port == s.Self.Port {
				continue
			}

			addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", vm.Hostname, vm.Port))
			if err != nil {
				log.Fatal(err)
			}

			_, err = s.Connection.WriteToUDP([]byte(msg), addr)
			if err != nil {
				log.Println(err)
				soleMember = false
				log.Printf("soleMember22%s n", soleMember)
				continue
			}
			log.Printf("Sent join request to %s:%d\n", vm.Hostname, vm.Port)
		}
		if soleMember {
			s.Self.IsLeader = true
			log.Printf("IsLeader Value of %s is changed to %t\n", s.Self.Hostname, s.Self.IsLeader)
		}
	} else {
		introducer := cluster[0]

		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", introducer.Hostname, introducer.Port))
		if err != nil {
			log.Fatal(err)
		}

		s.Connection.WriteToUDP([]byte(msg), addr)
		log.Printf("Sent join request to %s:%d\n", introducer.Hostname, introducer.Port)
	}

	s.TimerManager.RestartTimer(JOIN_TIMER_ID, JOIN_RETRY_TIMEOUT)
}
