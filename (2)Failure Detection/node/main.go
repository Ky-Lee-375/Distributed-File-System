// node/main.go

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type NodeDetails struct {
	ID           string    `json:"id"`
	IPAddress    string    `json:"ip_address"`
	Port         int       `json:"port"`
	Version      time.Time `json:"version"`
	Status       float32   `json:"status"` // 0.0 = OK | 1.0 = FAILED | 2.0 = LEFT | >3.0 = SUSPECTED
	LogicalClock int       `json:"logical_clock"`
}

const T_FAIL = 3
const K_PULL = 3

const DROP_PROB = 0.00

var ENABLE_SUSPICION = false

var membershipList = make(map[string][]NodeDetails)
var mu sync.Mutex

func main() {

	// Create the log directory if it does not exist
	logDir := "../log"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
	}

	// Create a log file in the log directory
	logFile, err := os.OpenFile(fmt.Sprintf("%s/NODE_%s.log", logDir, time.Now().Format("2006-01-02T15-04-05")), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %+v\n", err)
		return
	}
	defer logFile.Close()

	// Set the log output to the file
	log.SetOutput(logFile)

	idPtr := flag.String("id", "", "Node ID")
	ipPtr := flag.String("ip", "", "IP Address")
	portPtr := flag.Int("port", 9000, "Port number")
	introducerIdPtr := flag.String("introducer_ip", "", "Introducer IP")

	flag.Parse()

	if *idPtr == "" || *ipPtr == "" {
		fmt.Println("\033[1;93mID and IP address are required\033[0m")
		return
	}

	nodeDetails := NodeDetails{
		ID:           *idPtr,
		IPAddress:    *ipPtr,
		Port:         *portPtr,
		Version:      time.Now(),
		Status:       0.0,
		LogicalClock: 0,
	}

	log.Printf("NODE: Node has started\n")

	go startHTTPServer(portPtr)

	jsonData, err := json.Marshal(nodeDetails)
	if err != nil {
		fmt.Println(err)
		return
	}

	log.Printf("NODE: Node Details (Self) = %s\n", jsonData)

	introducerURL := fmt.Sprintf("http://%s:9000/node", *introducerIdPtr)
	fmt.Println(introducerURL)
	resp, err := http.Post(introducerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("\033[95mNode details sent to the introducer\033[0m")
	log.Printf("NODE: Node details sent to introducer\n")

	// Get the most up-to-date membership list
	resp, err = http.Get(introducerURL)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&membershipList); err != nil {
		fmt.Println(err)
		return
	}
	randSource := rand.NewSource(time.Now().UnixNano())
	randGen := rand.New(randSource)

	// change number for # of random nodes
	go pullMembership(nodeDetails, K_PULL, randGen)

	go listenForGossip(nodeDetails)

	go watchForCommands(nodeDetails)

	go incrementLogicalClock(&nodeDetails)

	select {}
}

func startHTTPServer(portPtr *int) {
	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var newNode NodeDetails
			if err := json.NewDecoder(r.Body).Decode(&newNode); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			mu.Lock()
			addMember(newNode, membershipList)
			mu.Unlock()

			log.Printf("NODE: Update membership list with %+v -> %+v\n", newNode, membershipList)
		} else {
			fmt.Println("\033[1;91mInvalid request method\033[0m")
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			log.Panic("Invalid request method")
		}
	})

	listenerPort := 9001 + *portPtr
	fmt.Printf("\033[1;34mNode HTTP server listening on port %d\033[0m\n", listenerPort)
	log.Printf("NODE: Node HTTP server listening on port %d", listenerPort)
	http.ListenAndServe(fmt.Sprintf(":%d", listenerPort), nil)
}

func pullMembership(nodeDetails NodeDetails, k int, randGen *rand.Rand) {
	for {
		// Filter out the failed nodes from the membership list
		activeNodes := make(map[string]NodeDetails)
		mu.Lock()
		for id, nodes := range membershipList {
			for _, node := range nodes {
				if node.Status != 1.0 && node.Status != 2.0 {
					activeNodes[id] = node
					break // break out of the inner loop once we have found an active node for this ID
				}
			}
		}
		mu.Unlock()
		log.Printf("NODE: Active Nodes: %+v\n", activeNodes)

		for _, chosenNode := range activeNodes {
			// Check if the chosen node is not the same as the current node
			if chosenNode.ID != nodeDetails.ID {
				go func(chosenNode NodeDetails) {
					conn, err := establishUDPConnection(chosenNode)
					if err != nil {
						log.Printf("NODE: Error establishing UDP connection: %+v\n", err)
						return
					}
					defer conn.Close()
					log.Printf("NODE: Established UDP connection with %+v\n", chosenNode)

					// Perform pull gossip using the established connection
					pullGossip(conn, chosenNode, nodeDetails)
				}(chosenNode)
			}
		}
		time.Sleep(time.Second * 2)
		log.Println("NODE: Sleeping for 2 seconds")
	}
}

func pullGossip(conn *net.UDPConn, targetNode NodeDetails, selfNodeDetails NodeDetails) {
	// Prepare gossip request with selfNodeDetails
	request := fmt.Sprintf("GOSSIP_REQUEST:")
	jsonSelfNode, err := json.Marshal(selfNodeDetails)
	if err != nil {
		fmt.Println("Error encoding selfNodeDetails:", err)
		log.Printf("NODE: Error encoding selfNodeDetails: %v\n", err)
		return
	}
	request += string(jsonSelfNode)

	totalDataSent := int64(len([]byte(request)))

	// Send the gossip request
	startTime := time.Now()
	_, err = conn.Write([]byte(request))
	if err != nil {
		fmt.Println("Error sending gossip request:", err)
		log.Printf("NODE: Error sending gossip request: %v\n", err)
		return
	}

	// Set a deadline for reading the response
	conn.SetReadDeadline(time.Now().Add(T_FAIL * time.Second))

	// Wait for a response or timeout
	buffer := make([]byte, 10240)
	n, _, err := conn.ReadFrom(buffer)
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	// Compute bandwidth in bits per second (bps)
	bandwidth := float64(totalDataSent*8) / elapsedTime.Seconds()
	log.Printf("NODE: Bandwidth: %f bps to %+v\n", bandwidth, targetNode)
	if err != nil {
		// Handle timeout or other read errors
		log.Printf("NODE: Error reading response or timeout occurred: %v\n", err)
		mu.Lock()
		for id, nodes := range membershipList {
			for i, memberNode := range nodes {
				if nodesMatch(targetNode, memberNode) {
					log.Printf("NODE: Handling suspicion/failure for node %s\n", memberNode.ID)
					if ENABLE_SUSPICION {
						if targetNode.Status == 0.0 {
							// Newly suspected
							memberNode.Status = 3.0
							fmt.Printf("Node %s is suspected.\n", memberNode.ID)
						} else if targetNode.Status >= 3.2 {
							// Conclude failure
							fmt.Printf("Node %s is failed. %s \n", memberNode.ID, time.Now())
							memberNode.Status = 1.0
						} else if targetNode.Status >= 3.0 {
							// Already suspected -> increment 0.1
							memberNode.Status += 0.1
						}
					} else {
						// Update the status of the target node to failed
						fmt.Printf("Node %s is failed. %s \n", memberNode.ID, time.Now())
						memberNode.Status = 1.0
					}
					membershipList[id][i] = memberNode
					break
				}
			}
		}
		mu.Unlock()
		return
	}

	// Handle the received membership list
	var receivedList map[string][]NodeDetails
	if err := json.Unmarshal(buffer[:n], &receivedList); err != nil {
		fmt.Println("Error unmarshalling receivedList:", err)
		log.Printf("NODE: Error unmarshalling receivedList: %v\n", err)
		return
	}
	log.Printf("NODE: Received membership list: %+v\n", receivedList)

	logicalTime, err := getLogicalTime(selfNodeDetails)
	if err != nil {
		fmt.Println("Error getting logical time(pull gossip):", err)
		log.Printf("NODE: Error getting logical time(pull gossip): %v\n", err)
		// handle the error appropriately, for example, continue to the next iteration
		return
	}

	mu.Lock()
	// Find the target node in the membership list and update its logical time value
	log.Printf("NODE: Merging received membership list with local list\n")
	for id, nodes := range membershipList {
		for i, memberNode := range nodes {
			if nodesMatch(targetNode, memberNode) {
				// Update the logical clock of the matching target node
				memberNode.LogicalClock = logicalTime
				memberNode.Status = 0.0
				// Update the node in the membership list
				membershipList[id][i] = memberNode
				break
			}
		}
	}
	mu.Unlock()

	// Iterate through the received list and compare with the local membership list
	mu.Lock()
	defer mu.Unlock()
	for id, receivedNodes := range receivedList {
		if existingNodes, exists := membershipList[id]; !exists {
			// ID does not exist in the local list, so add all received nodes for this ID
			for _, receivedNode := range receivedNodes {
				receivedNode.Status = 0.0
				receivedNode.LogicalClock = 0
			}
			membershipList[id] = receivedNodes
		} else {
			// ID exists, check each received node for matching version
			for _, receivedNode := range receivedNodes {
				matchFound := false
				for _, localNode := range existingNodes {
					if nodesMatch(localNode, receivedNode) {
						matchFound = true
						break
					}
				}
				if !matchFound {
					// No matching version found, add the receivedNode as a new version
					receivedNode.Status = 0.0
					receivedNode.LogicalClock = 0
					membershipList[id] = append(membershipList[id], receivedNode)
				}
			}
		}
	}
}

func sendLeavingNotification(selfNode NodeDetails) {
	log.Printf("NODE: Sending leaving notification for %+v\n", selfNode)

	mu.Lock()
	defer mu.Unlock()

	for _, nodes := range membershipList {
		for _, node := range nodes {
			conn, err := establishUDPConnection(node)
			if err != nil {
				log.Printf("NODE: Error establishing UDP connection to %+v: %v\n", node, err)
				continue
			}
			defer conn.Close()

			// Prepare leaving notification with selfNodeDetails
			notification := "LEAVING:"
			jsonSelfNode, err := json.Marshal(selfNode)
			if err != nil {
				log.Printf("NODE: Error encoding selfNodeDetails: %v\n", err)
				return
			}
			notification += string(jsonSelfNode)

			// Send the leaving notification
			_, err = conn.Write([]byte(notification))
			if err != nil {
				log.Printf("NODE: Error sending leaving notification to %+v: %v\n", node, err)
			} else {
				log.Printf("NODE: Successfully sent leaving notification to %+v\n", node)
			}
		}
	}
}

func listenForGossip(selfNode NodeDetails) {
	log.Printf("NODE: Listening for gossip on port %d\n", selfNode.Port)

	// Use 0.0.0.0 instead of a specific IP address
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", selfNode.Port))
	if err != nil {
		log.Printf("NODE: Error resolving UDP address: %v\n", err)
		return
	}

	// Create UDP connection
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("NODE: Error listening on UDP: %v\n", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 10240)

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("NODE: Error reading from UDP: %v\n", err)
			return
		}

		message := string(buffer[:n])
		log.Printf("NODE: Received message: %s from %v\n", message, addr)

		if strings.HasPrefix(message, "LEAVING:") {
			// Handle the leaving notification
			log.Println("NODE: Handling LEAVING message")

			// Trim the prefix
			trimmedMessage := strings.TrimPrefix(message, "LEAVING:")

			// Unmarshal the trimmed message into NodeDetails structure
			var leavingNode NodeDetails
			err := json.Unmarshal([]byte(trimmedMessage), &leavingNode)
			if err != nil {
				log.Printf("NODE: Error decoding leaving node details: %v\n", err)
				continue
			}

			mu.Lock()
			for id, nodes := range membershipList {
				for i, memberNode := range nodes {
					if nodesMatch(leavingNode, memberNode) {
						// Update the status of the leaving node in the membership list
						memberNode.Status = 2.0
						membershipList[id][i] = memberNode
						log.Printf("NODE: Updated status of leaving node: %+v\n", memberNode)
						break
					}
				}
			}
			mu.Unlock()
		}

		// Create a new rand.Rand object with a source seeded by the current time
		randSource := rand.NewSource(time.Now().UnixNano())
		randGen := rand.New(randSource)

		// Generate a random floating-point number between 0 and 1
		randProb := randGen.Float64()

		if randProb < DROP_PROB {
			log.Printf("NODE: Message dropped with probability %f\n", randProb)
		} else {
			log.Printf("NODE: Message passed with probability %f\n", randProb)

			if strings.HasPrefix(message, "GOSSIP_REQUEST:") {
				// Handle the request
				log.Println("NODE: Handling GOSSIP_REQUEST message")

				// Trim the prefix
				trimmedMessage := strings.TrimPrefix(message, "GOSSIP_REQUEST:")

				// Unmarshal the trimmed message into NodeDetails structure
				var nodeDetails NodeDetails
				err := json.Unmarshal([]byte(trimmedMessage), &nodeDetails)
				if err != nil {
					log.Printf("NODE: Error decoding node details: %v\n", err)
					return
				}

				mu.Lock()
				matchFound := false
				for _, nodes := range membershipList {
					for _, memberNode := range nodes {
						if memberNode.ID == nodeDetails.ID &&
							memberNode.IPAddress == nodeDetails.IPAddress &&
							memberNode.Port == nodeDetails.Port &&
							memberNode.Version == nodeDetails.Version {
							matchFound = true
							break
						}
					}
					if matchFound {
						break
					}
				}
				mu.Unlock()

				if !matchFound {
					// Add a new node to the membership list with LogicalClock as 0 and Status as 0.0
					log.Printf("NODE: Adding new node to membership list: %+v\n", nodeDetails)

					logicalTime, err := getLogicalTime(selfNode)
					if err != nil {
						log.Printf("NODE: Error getting logical time(listen): %v\n", err)
						continue
					}
					newNode := NodeDetails{
						ID:           nodeDetails.ID,
						IPAddress:    nodeDetails.IPAddress,
						Port:         nodeDetails.Port,
						Version:      nodeDetails.Version,
						LogicalClock: logicalTime,
						Status:       0.0,
					}

					mu.Lock()
					if nodes, exists := membershipList[newNode.ID]; exists {
						membershipList[newNode.ID] = append(nodes, newNode)
					} else {
						membershipList[newNode.ID] = []NodeDetails{newNode}
					}
					mu.Unlock()
				}

				jsonBytes, err := json.Marshal(membershipList)
				if err != nil {
					log.Printf("NODE: Error encoding membershipList: %v\n", err)
					return
				}

				_, err = conn.WriteToUDP(jsonBytes, addr)
				if err != nil {
					log.Printf("NODE: Error sending back the membership List: %v\n", err)
					return
				}
			}

			if message == "CHECK" {
				log.Println("NODE: Handling CHECK message")

				// Respond back to the introducer
				_, err := conn.WriteToUDP([]byte("ALIVE"), addr)
				if err != nil {
					log.Printf("NODE: Error responding to CHECK message: %v\n", err)
				}
			}
		}
		if strings.HasPrefix(message, "SUSPICION_CHANGE:") {
			log.Println("NODE: Handling SUSPICION_CHANGE message")

			trimmedMessage := strings.TrimPrefix(message, "SUSPICION_CHANGE:")
			suspicionValue, err := strconv.ParseBool(trimmedMessage)
			if err != nil {
				log.Printf("NODE: Error parsing suspicion value: %v\n", err)
			} else {
				ENABLE_SUSPICION = suspicionValue
				log.Printf("NODE: ENABLE_SUSPICION Value has changed to %v\n", ENABLE_SUSPICION)
			}
		}
	}
}
