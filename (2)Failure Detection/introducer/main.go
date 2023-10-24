// introducer/main.go

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
)

type Node struct {
	ID        string `json:"id"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port"`
	Version   string `json:"version"`
}

var membershipList = make(map[string][]Node)
var mu sync.Mutex

func main() {

	// Create the log directory if it does not exist
	logDir := "../log"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
	}

	// Create a log file in the log directory
	logFile, err := os.OpenFile(fmt.Sprintf("%s/INTRODUCER_%s.log", logDir, time.Now().Format("2006-01-02T15-04-05")), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer logFile.Close()

	// Set the log output to the file
	log.SetOutput(logFile)

	// Define the flag
	ipPtr := flag.String("ip", "localhost", "IP Address where the introducer is running")

	// Parse the flag
	flag.Parse()

	log.Printf("INTRODUCER: Introducer has started\n")

	// Start the HTTP server in a separate goroutine
	go startHTTPServer(*ipPtr)

	go checkMembers()

	// Prevent the main function from exiting
	select {}
}

func startHTTPServer(ipAddress string) {
	http.HandleFunc("/node", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var node Node
			if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			jsonData, err := json.Marshal(node)
			if err != nil {
				fmt.Println("Error marshalling JSON:", err)
				return
			}
			log.Printf("INTRODUCER: Node joined: %s\n", jsonData)
			mu.Lock()
			existingNodes, exists := membershipList[node.ID]

			if exists {
				found := false
				log.Printf("INTRODUCER: Node is already existing - adding version\n")
				for _, existingNode := range existingNodes {
					if nodesMatch(node, existingNode) {
						found = true
						break
					}
				}

				if !found {
					membershipList[node.ID] = append(existingNodes, node)
				}
			} else {
				log.Printf("INTRODUCER: Node is not existing - adding key value\n")
				membershipList[node.ID] = []Node{node}
				fmt.Println()
			}
			mu.Unlock()
			mu.Lock()
			printMembershipList()
			mu.Unlock()
		} else if r.Method == http.MethodGet {
			mu.Lock()
			list, err := json.Marshal(membershipList)
			log.Printf("INTRODUCER: GET request - sending back membership list: %s\n", list)
			mu.Unlock()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(list)
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	fmt.Printf("\033[1;35mListening for nodes on %s:%d\033[0m\n", ipAddress, 9000)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", ipAddress, 9000), nil))
}

func notifyNode(member Node, newNode Node) {
	jsonData, err := json.Marshal(newNode)
	if err != nil {
		fmt.Println(err)
		return
	}

	nodeUpdateURL := fmt.Sprintf("http://%s:%d/update", member.IPAddress, 9001+member.Port)

	log.Printf("INTRODUCER: Notify node %s to %s\n", jsonData, nodeUpdateURL)

	resp, err := http.Post(nodeUpdateURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
}

func nodesMatch(node1, node2 Node) bool {
	return node1.ID == node2.ID &&
		node1.IPAddress == node2.IPAddress &&
		node1.Port == node2.Port &&
		node1.Version == node2.Version
}

func printMembershipList() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "IP Address", "Port", "Version"})

	for id, nodes := range membershipList {
		for _, node := range nodes {
			row := []string{
				id,
				node.IPAddress,
				fmt.Sprintf("%d", node.Port),
				node.Version,
			}
			table.Append(row)
		}
	}

	// Clear the screen and print the updated table
	fmt.Print("\033[H\033[2J")
	table.Render()
}

func checkMembers() {
	for {
		mu.Lock()
		for _, nodes := range membershipList {
			for _, node := range nodes {
				ip := net.ParseIP(node.IPAddress)
				if ip == nil {
					log.Printf("INTRODUCER: Invalid IP address: %s\n", node.IPAddress)
					continue
				}

				addr := net.UDPAddr{
					Port: node.Port, // Use the correct port
					IP:   ip,
				}
				conn, err := net.DialUDP("udp", nil, &addr)
				if err != nil {
					log.Println(err)
					continue
				}

				// Send CHECK message
				_, err = conn.Write([]byte("CHECK"))
				if err != nil {
					log.Println(err)
					continue
				}
				log.Printf("INTRODUCER: Checking node %s at %s %d\n", node.ID, addr.IP, addr.Port)

				// Set read deadline
				conn.SetReadDeadline(time.Now().Add(2 * time.Second))

				// Wait for ALIVE response
				buffer := make([]byte, 10240)
				_, _, err = conn.ReadFromUDP(buffer)
				if err != nil {
					// Handle timeout or connection error
					// log.Println("Node not responding:", node.ID)
					// Remove the node from membershipList
					log.Printf("INTRODUCER: Checking node %s at %s is DEAD\n", node.ID, node.IPAddress)
					removeNodeFromList(node)
				} else {
					log.Printf("INTRODUCER: Checking node %s at %s is ALIVE\n", node.ID, node.IPAddress)
				}

				conn.Close()
			}
		}
		mu.Unlock()

		mu.Lock()
		printMembershipList()
		mu.Unlock()

		time.Sleep(5 * time.Second) // Adjust the sleep duration as needed
	}
}

func removeNodeFromList(node Node) {
	// Find and remove the node from membershipList
	for id, nodes := range membershipList {
		for i, memberNode := range nodes {
			if nodesMatch(node, memberNode) {
				// Remove the node
				membershipList[id] = append(nodes[:i], nodes[i+1:]...)
				break
			}
		}
	}
}
