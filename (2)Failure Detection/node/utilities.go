// node/utilities.go

package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

// min returns the smaller of the two integer values.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// clearScreen clears the terminal screen.
func clearScreen() {
	switch platform := runtime.GOOS; platform {
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default: // For Linux, Unix, and MacOS
		fmt.Print("\033[H\033[2J")
	}
}

// abs returns the absolute value of the integer x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// nodesMatch compares two NodeDetails and returns true if they match.
func nodesMatch(node1, node2 NodeDetails) bool {
	return node1.ID == node2.ID &&
		node1.IPAddress == node2.IPAddress &&
		node1.Port == node2.Port &&
		node1.Version.Equal(node2.Version)
}

// establishUDPConnection establishes a UDP connection to the target node and returns the connection.
func establishUDPConnection(targetNode NodeDetails) (*net.UDPConn, error) {
	log.Printf("NODE: Establishing UDP connection to %v\n", targetNode)
	
	// Build the address string for the UDP connection
	address := fmt.Sprintf("%s:%d", targetNode.IPAddress, targetNode.Port)

	// Resolve UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		log.Printf("NODE: Error resolving UDP address: %v\n", err)
		return nil, err
	}

	// Dial a UDP connection
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Printf("NODE: Error dialing UDP connection: %v\n", err)
		return nil, err
	}
	return conn, nil
}

func chooseRandomNodes(membershipList map[string]NodeDetails, nodeID string, k int, randGen *rand.Rand) []NodeDetails {
	log.Println("NODE: Choosing random nodes from the membership list")
	nodes := make([]NodeDetails, 0, len(membershipList))

	// Populate the nodes slice excluding the node itself
	for id, node := range membershipList {
		if id != nodeID {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) <= k {
		return nodes
	}

	// Shuffle the nodes
	randGen.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })

	// Return the first k nodes
	return nodes[:min(k, len(nodes))]
}

func printMembershipList() {
	log.Println("NODE: Printing current membership list")
	mu.Lock()
	defer mu.Unlock()
	fmt.Println("\033[1;97mCurrent membership list:\033[0m")

	// Create a tablewriter instance
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "IP Address", "Port", "Version", "Status", "Logical Clock"})

	// Collect and sort the keys (node IDs)
	keys := make([]string, 0, len(membershipList))
	for k := range membershipList {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Iterate over the sorted keys
	for _, key := range keys {
		nodes := membershipList[key]

		// Iterate over each node in the slice
		for _, node := range nodes {
			// Determine the color for the Status field based on its value
			var statusColor string
			switch {
			case node.Status == 0.0:
				statusColor = "\033[32m" // Green
			case node.Status == 1.0:
				statusColor = "\033[31m" // Red
			case node.Status == 2.0:
				statusColor = "\033[34m" // Blue
			case node.Status >= 3.0:
				statusColor = "\033[33m" // Orange
			default:
				statusColor = "\033[0m" // Default color
			}

			row := []string{
				node.ID,
				node.IPAddress,
				fmt.Sprintf("%d", node.Port),
				node.Version.Format(time.RFC3339),
				fmt.Sprintf("%s%f\033[0m", statusColor, node.Status), // Apply color to Status
				fmt.Sprintf("%d", node.LogicalClock),
			}
			table.Append(row)
		}
	}
	table.Render() // Render the table
}

func watchForCommands(nodeDetails NodeDetails) {
	log.Println("NODE: Watching for commands")
	reader := bufio.NewReader(os.Stdin)

	for {
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("NODE: Error reading command: %v\n", err)
			continue
		}

		text = strings.TrimSpace(text)
		log.Printf("NODE: Received command: %s\n", text)
		switch text {
		case "list_mem":
			printMembershipList()
		case "list_self":
			fmt.Println(nodeDetails.ID)
		case "enable_sus":
			ENABLE_SUSPICION = true
			checkSuspicionChange(&membershipList, &nodeDetails)
		case "disable_sus":
			ENABLE_SUSPICION = false
			checkSuspicionChange(&membershipList, &nodeDetails)
		case "leave":
			sendLeavingNotification(nodeDetails)
			fmt.Println("\033[1;34mNode is leaving, terminating program.\033[0m")
			os.Exit(0)
		}
	}
}

// CLOCK FUNCTIONS

func incrementLogicalClock(nodeDetails *NodeDetails) {
	log.Println("NODE: Incrementing logical clock")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		mu.Lock() // Lock the mutex before accessing the shared membershipList
		for id, nodes := range membershipList {
			for i, memberNode := range nodes {
				if nodesMatch(*nodeDetails, memberNode) {
					// Increment the logical clock of the matching node
					memberNode.LogicalClock++
					// Update the node in the membership list
					membershipList[id][i] = memberNode
					break
				}
			}
		}
		mu.Unlock() // Unlock the mutex after modifying the membershipList
	}
}

func getLogicalTime(nodeDetails NodeDetails) (int, error) {
	log.Printf("NODE: Getting logical time for %+v\n", nodeDetails)
	mu.Lock()
	defer mu.Unlock()
	for _, nodes := range membershipList {
		for _, memberNode := range nodes {
			if nodesMatch(nodeDetails, memberNode) {
				return memberNode.LogicalClock, nil
			}
		}
	}

	return 0, fmt.Errorf("Matching node not found in the membership list")
}

func addMember(newNode NodeDetails, membershipList map[string][]NodeDetails) {
	log.Printf("NODE: Adding member %+v to membership list\n", newNode)
	mu.Lock()
	defer mu.Unlock()

	// Check if a node with the same ID exists in the membershipList
	existingNodes, exists := membershipList[newNode.ID]

	if exists {
		// Check if a node with matching details already exists in the list
		found := false
		for _, node := range existingNodes {
			if nodesMatch(node, newNode) {
				// Node with matching details found, no need to add a new version
				found = true
				break
			}
		}

		// If no matching node is found, append the new node to the existing list
		if !found {
			membershipList[newNode.ID] = append(existingNodes, newNode)
		}
	} else {
		// If no node with the same ID exists, create a new entry in the map
		membershipList[newNode.ID] = []NodeDetails{newNode}
	}
}

func checkSuspicionChange(membershipList *map[string][]NodeDetails, selfNode *NodeDetails) {
	log.Printf("NODE: Checking suspicion change\n")
	for _, nodes := range *membershipList {
		for _, targetNode := range nodes {
			// Check if targetNode is not itself
			if targetNode.ID == selfNode.ID && targetNode.IPAddress == selfNode.IPAddress && targetNode.Port == selfNode.Port {
				continue
			}

			conn, err := establishUDPConnection(targetNode)
			if err != nil {
				log.Printf("NODE: Error establishing UDP connection: %v\n", err)
				continue // Continue to the next node on error
			}
			defer conn.Close()

			// Prepare message for suspicion change notification
			message := fmt.Sprintf("SUSPICION_CHANGE:%t", ENABLE_SUSPICION)

			// Send the suspicion change message
			_, err = conn.Write([]byte(message))
			if err != nil {
				log.Printf("NODE: Error sending suspicion change message: %v\n", err)
			}
		}
	}
}
