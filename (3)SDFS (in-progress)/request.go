package main

import (
	"bytes"
	"cs425/server"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type OperationType int

const (
	PUT    OperationType = iota // Insert or update a file
	GET                         // Retrieve a file
	DELETE                      // Delete a file
	LS                          // List VM addresses for a file
	STORE                       // List files stored at a machine
)

type SDFSRequest struct {
	Operation      OperationType
	LocalFilename  string
	SDFSFilename   string
	RequesterVm    string
	IsWriteRequest bool
	IsReadRequest  bool
	Body           []byte
}

func NewSDFSRequest(op OperationType, local, sdfs string, s *server.Host) *SDFSRequest {
	// Read and encode the file's content if it's a PUT request
	var bodyContent []byte
	if op == PUT {
		content, err := ioutil.ReadFile(local)
		if err != nil {
			log.Errorf("Failed to read file %s: %v", local, err)
			return nil
		}
		bodyContent = content
	}

	return &SDFSRequest{
		Operation:      op,
		LocalFilename:  local,
		SDFSFilename:   sdfs,
		RequesterVm:    "http://" + s.Hostname + s.Address.String(),
		IsWriteRequest: op == PUT,
		IsReadRequest:  op == GET,
		Body:           bodyContent,
	}
}

// Usage
// request := NewSDFSRequest(PUT, "local.txt", "sdfs.txt", "192.168.0.1")
// ... use the request ...

func HandleCommand(s *server.Server, command string) {
	commands := []string{
		"ls-mem: print membership table",
		"id: print id of node",
		"kill: crash server",
		"join: start gossiping",
		"leave: stop gossiping",
		"sus (on|off): enable/disable gossip suspicion protocol",
		"put <localfilename> <sdfsfilename>: insert/update a file in the SDFS",
		"get <sdfsfilename> <localfilename>: fetch a file from SDFS to local directory",
		"delete <sdfsfilename>: delete a file from SDFS",
		"ls <sdfsfilename>: list all VM addresses where this file is stored in SDFS",
		"store: list all files currently stored on this machine in SDFS",
		"help: list all commands",
	}

	// Split the command string into words
	args := strings.Fields(command)

	if len(args) == 0 {
		fmt.Println("Empty command received.")
		return
	}

	// Switch on the first word of the command
	switch strings.ToLower(args[0]) {

	case "ls-mem":
		s.PrintMembershipTable()

	case "id":
		fmt.Println(s.Self.ID)

	case "kill":
		log.Fatalf("Kill command received at %d milliseconds", time.Now().UnixMilli())

	case "start_gossip":
		fallthrough

	case "join":
		startGossip(s)
		fmt.Println("OK")

	case "stop_gossip":
		fallthrough

	case "leave":
		stopGossip(s)
		fmt.Println("OK")

	case "help":
		for i := range commands {
			fmt.Printf("%d. %s\n", i+1, commands[i])
		}

	case "put":
		if len(args) != 3 {
			fmt.Println("Usage: put localfilename sdfsfilename")
			return
		}
		putFile(s, args[1], args[2])

	case "get":
		if len(args) != 3 {
			fmt.Println("Usage: get sdfsfilename localfilename")
			return
		}
		getFile(s, args[1], args[2])

	case "delete":
		if len(args) != 2 {
			fmt.Println("Usage: delete sdfsfilename")
			return
		}
		deleteFile(s, args[1])

	case "ls":
		if len(args) != 2 {
			fmt.Println("Usage: ls sdfsfilename")
			return
		}
		listVMsForFile(s, args[1])

	case "store":
		listStoredFiles(s)

	}

}

func getLeader(s *server.Server) string {
	for _, host := range s.Members {
		if host.IsLeader {
			// Assemble the leader's URL
			return fmt.Sprintf("http://%s:%d/receiveRequest", host.Address.IP.String(), host.Port+100)
		}
	}
	return "" // Return empty string if no leader found
}

func sendRequestToLeader(s *server.Server, request *SDFSRequest) (string, error) {
	// Serialize the SDFSRequest to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error serializing request: %v", err)
	}

	// Find the Leader from the membership list and assemble URL
	url := getLeader(s)
	if url == "" {
		return "", fmt.Errorf("no leader found")
	}

	// log.Printf("Sending SDFSRequest message: %s to URL: %s\n", string(jsonData), url)

	// Send the JSON data as a POST request to the specified IP address
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error sending request to %s: %v", url, err)
	}
	defer resp.Body.Close()

	// Handle non-200 OK responses here
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK response: %s", resp.Status)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}
	responseBody := string(bodyBytes)

	if responseBody != "Request successfully enqueued" {
		return "", fmt.Errorf("unexpected response: %s", responseBody)
	}

	return responseBody, nil
}

func putFile(s *server.Server, localFilename, sdfsFilename string) {
	// Create a new SDFS request for the put operation
	request := NewSDFSRequest(PUT, localFilename, sdfsFilename, s.Self)

	// If the request couldn't be created (e.g., due to a file read error), exit
	if request == nil {
		return
	}

	// Send request to SDFS coordinator or handler to decide where and how to store the file.
	// This would typically involve finding nodes that have space, breaking the file into chunks if necessary,
	// and sending those chunks to the chosen nodes. For simplicity, I'll just mock the process here.

	response, err := sendRequestToLeader(s, request)
	if err != nil {
		fmt.Printf("Error while putting file to SDFS: %s\n", err)
		return
	}

	if response == "Request successfully enqueued" {
		fmt.Printf("Request to store file %s in SDFS as %s successfully enqueued\n", localFilename, sdfsFilename)
	} else {
		fmt.Printf("Failed to enqueue request: store file %s in SDFS: %s\n", localFilename, response)
	}
}

func getFile(s *server.Server, sdfsFilename, localFilename string) {
	// Create a new SDFS request for the put operation
	request := NewSDFSRequest(GET, localFilename, sdfsFilename, s.Self)

	// If the request couldn't be created (e.g., due to a file read error), exit
	if request == nil {
		return
	}

	// Send request to SDFS coordinator or handler to decide where and how to store the file.
	// This would typically involve finding nodes that have space, breaking the file into chunks if necessary,
	// and sending those chunks to the chosen nodes. For simplicity, I'll just mock the process here.

	response, err := sendRequestToLeader(s, request)
	if err != nil {
		fmt.Printf("Error while getting file from SDFS: %s\n", err)
		return
	}

	if response == "Request successfully enqueued" {
		fmt.Printf("Request to get file %s in SDFS as %s successfully enqueued\n", localFilename, sdfsFilename)
	} else {
		fmt.Printf("Failed to enqueue request: store file %s in SDFS: %s\n", localFilename, response)
	}
}

func deleteFile(s *server.Server, sdfsFilename string) {
	// Create a new SDFS request for the put operation
	request := NewSDFSRequest(GET, "", sdfsFilename, s.Self)

	// If the request couldn't be created (e.g., due to a file read error), exit
	if request == nil {
		return
	}

	// Send request to SDFS coordinator or handler to decide where and how to store the file.
	// This would typically involve finding nodes that have space, breaking the file into chunks if necessary,
	// and sending those chunks to the chosen nodes. For simplicity, I'll just mock the process here.

	response, err := sendRequestToLeader(s, request)
	if err != nil {
		fmt.Printf("Error while deleting file from SDFS: %s\n", err)
		return
	}

	if response == "Request successfully enqueued" {
		fmt.Printf("Request to delete file %s in SDFS successfully enqueued\n", sdfsFilename)
	} else {
		fmt.Printf("Failed to enqueue request: delete file %s in SDFS: %s\n", sdfsFilename, response)
	}
}

func listVMsForFile(s *server.Server, sdfsFilename string) {
	// Implementation for "ls" operation here
}

func listStoredFiles(s *server.Server) {
	// Implementation for "store" operation here
}

// Handles the request received by the server
// JOIN, PING, ID, LIST, KILL, START_GOSSIP, STOP_GOSSIP, CONFIG, SUS ON, SUS OFF, LIST_SUS
func HandleRequest(s *server.Server, e server.ReceiverEvent) {
	commands := []string{"ls: print membership table", "id: print id of node",
		"kill: crash server", "start_gossip: start gossiping", "stop_gossip: stop gossiping", "sus <on|off>: toggle gossip suspicion protocol",
		"config <option> [<value>]: get/set config parameter", "help: list all commands"}

	lines := strings.Split(e.Message, "\n")
	if len(lines) < 1 {
		return
	}

	header := lines[0]
	tokens := strings.Split(header, " ")

	log.Debugf("Request %s received from: %v\n", tokens[0], e.Sender)

	switch verb := strings.ToLower(tokens[0]); verb {
	case "join":
		HandleJoinRequest(s, e)

	case "join_ok":
		HandleJoinResponse(s, e)

	case "join_error":
		log.Warnf("Failed to join: %s", e.Message)

	case "ping":
		HandlePingRequest(s, e)

	case "id":
		s.Connection.WriteToUDP([]byte(fmt.Sprintf("%s\n", s.Self.ID)), e.Sender)

	case "ls":
		s.Connection.WriteToUDP([]byte(fmt.Sprintf("OK\n%s\n", strings.ReplaceAll(s.EncodeMembersList(), ";", "\n"))), e.Sender)

	case "kill":
		log.Fatalf("KILL command received at %d milliseconds", time.Now().UnixMilli())

	case "start_gossip":
		startGossip(s)
		log.Warnf("START command received at %d milliseconds", time.Now().UnixMilli())
		s.Connection.WriteToUDP([]byte("OK\n"), e.Sender)

	case "stop_gossip":
		stopGossip(s)
		log.Warnf("STOP command received at %d milliseconds", time.Now().UnixMilli())
		s.Connection.WriteToUDP([]byte("OK\n"), e.Sender)

	case "config":
		HandleConfigRequest(s, e)

	case "sus":
		HandleSusRequest(s, e)

	case "help":
		s.Connection.WriteToUDP([]byte(strings.Join(commands, "\n")), e.Sender)

	default:
		log.Warn("Unknown request verb: ", verb)
	}
}

func HandleJoinResponse(s *server.Server, e server.ReceiverEvent) {
	lines := strings.Split(e.Message, "\n")
	if len(lines) < 2 || (s.Active && !IsIntroducer(s)) {
		return
	}

	log.Info("Join accepted by ", e.Sender)

	s.TimerManager.StopTimer(JOIN_TIMER_ID)
	s.ProcessMembersList(lines[1])
	// s.StartAllTimers()
	s.Active = true
	log.Info("Node join completed.")
}

// Handle config command: CONFIG <field to change> <value>
func HandleConfigRequest(s *server.Server, e server.ReceiverEvent) {
	words := strings.Split(e.Message, " ")

	if words[1] == "DROPRATE" {
		if len(words) == 2 {
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("DROPRATE %d\n", s.DropRate)), e.Sender)
		} else if len(words) == 3 {
			dropRate, err := strconv.Atoi(words[2])
			if err != nil {
				return
			}
			s.DropRate = dropRate
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("OK\n")), e.Sender)
		}
	}

	if words[1] == "T_GOSSIP" {
		if len(words) == 2 {
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("T_GOSSIP %d ms\n", server.T_GOSSIP.Milliseconds())), e.Sender)
		} else if len(words) == 3 {
			value, err := strconv.Atoi(words[2])
			if err != nil {
				return
			}
			server.T_GOSSIP = time.Duration(value) * time.Millisecond
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("OK\n")), e.Sender)
		}
	}

	if words[1] == "T_FAIL" {
		if len(words) == 2 {
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("T_FAIL %d ms\n", server.T_FAIL.Milliseconds())), e.Sender)
		} else if len(words) == 3 {
			value, err := strconv.Atoi(words[2])
			if err != nil {
				return
			}
			server.T_FAIL = time.Duration(value) * time.Millisecond
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("OK\n")), e.Sender)
		}
	}

	if words[1] == "T_SUSPECT" {
		if len(words) == 2 {
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("T_SUSPECT %d ms\n", server.T_SUSPECT.Milliseconds())), e.Sender)
		} else if len(words) == 3 {
			value, err := strconv.Atoi(words[2])
			if err != nil {
				return
			}
			server.T_SUSPECT = time.Duration(value) * time.Millisecond
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("OK\n")), e.Sender)
		}
	}

	if words[1] == "T_CLEANUP" {
		if len(words) == 2 {
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("T_CLEANUP %d ms\n", server.T_CLEANUP.Milliseconds())), e.Sender)
		} else if len(words) == 3 {
			value, err := strconv.Atoi(words[2])
			if err != nil {
				return
			}
			server.T_CLEANUP = time.Duration(value) * time.Millisecond
			s.Connection.WriteToUDP([]byte(fmt.Sprintf("OK\n")), e.Sender)
		}
	}
}

// Received member list from peer
func HandlePingRequest(s *server.Server, e server.ReceiverEvent) {
	if !s.Active {
		log.Debugf("PING from %s dropped as server is inactive\n", e)
		return
	}

	lines := strings.Split(e.Message, "\n")
	if len(lines) < 2 {
		return
	}

	tokens := strings.Split(lines[0], " ")
	if len(tokens) < 3 {
		log.Debugf("Illegal header for PING request: %s\n", lines[0])
		return
	}

	if rand.Intn(100) < s.DropRate {
		log.Debugf("PING from %s dropped with drop rate %d %%\n", e, s.DropRate)
		return
	}

	if tokens[2] != s.Self.ID {
		log.Debugf("Dropped PING due to ID mismatch: %s\n", tokens[2])
		return
	}

	s.ProcessMembersList(lines[1])
}

// Change gossip protocol
func HandleSusRequest(s *server.Server, e server.ReceiverEvent) {
	lines := strings.Split(e.Message, "\n")
	tokens := strings.Split(lines[0], " ")
	if len(tokens) < 2 {
		s.Connection.WriteToUDP([]byte("ERROR\n"), e.Sender)
		return
	}

	if strings.ToUpper(tokens[1]) == "ON" {
		s.ChangeProtocol(server.GOSPSIP_SUSPICION_PROTOCOL)
	} else if strings.ToUpper(tokens[1]) == "OFF" {
		s.ChangeProtocol(server.GOSSIP_PROTOCOL)
	}

	s.Connection.WriteToUDP([]byte("OK\n"), e.Sender)
}

// Function to handle the Join request by new node at any node
func HandleJoinRequest(s *server.Server, e server.ReceiverEvent) {
	if !s.Active && !IsIntroducer(s) {
		return
	}

	message := e.Message
	lines := strings.Split(message, "\n")
	tokens := strings.Split(lines[0], " ")
	if len(tokens) < 2 {
		s.Connection.WriteToUDP([]byte(ERROR_ILLEGAL_REQUEST), e.Sender)
		return
	}

	senderInfo := tokens[1]
	tokens = strings.Split(senderInfo, ":")
	if len(tokens) < 3 {
		s.Connection.WriteToUDP([]byte(ERROR_ILLEGAL_REQUEST), e.Sender)
		return
	}

	senderAddress, senderPort, senderId := tokens[0], tokens[1], tokens[2]
	senderPortInt, err := strconv.Atoi(senderPort)
	if err != nil {
		s.Connection.WriteToUDP([]byte(ERROR_ILLEGAL_REQUEST), e.Sender)
		return
	}

	host, err := s.AddHost(senderAddress, senderPortInt, senderId)
	if err != nil {
		log.Errorf("Failed to add host: %s\n", err.Error())
		reply := fmt.Sprintf("%s\n%s\n", JOIN_ERROR, err.Error())
		s.Connection.WriteToUDP([]byte(reply), e.Sender)
		return
	}

	reply := fmt.Sprintf("%s\n%s\n", JOIN_OK, s.EncodeMembersList())
	_, err = s.Connection.WriteToUDP([]byte(reply), host.Address)
	if err != nil {
		log.Error(err)
	}
}
