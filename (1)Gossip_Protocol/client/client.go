package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// handleConnection manages communication with a connected Server(s).
func handleConnection(conn net.Conn, Servers map[string]net.Conn, timeStamps map[string]time.Time) {
	defer conn.Close()

	addr := conn.RemoteAddr().String()
	fmt.Printf("\033[1;32mNew connection: %s\033[0m\n", addr)
	Servers[addr] = conn

	buf := make([]byte, 8192)

	for {
		// Read data from the connected Server
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("\033[1;38;5;208mConnection from %s closed: %s\033[0m\n", addr, err)
			delete(Servers, addr) // Remove Server from the map
			break
		}

		message := string(buf[:n])

		fmt.Printf("\033[1;36mReceived from %s:\033[0m\n%s", addr, message)

		// if prefix grep
		// send message to all others "Client execute: [message]"
		// send output to grep requester

		if timestamp, ok := timeStamps[addr]; ok {
			duration := time.Since(timestamp)
			fmt.Printf("\033[1;33mLatency %s: %v\033[0m\n", addr, duration)
			delete(timeStamps, addr)
		}

		// Relay the received message to all connected Servers except the sender
		for ServerAddr, ServerConn := range Servers {
			if ServerAddr != addr {
				_, err := ServerConn.Write([]byte(addr + ": " + message))
				if err != nil {
					fmt.Printf("\033[1;31mError relaying message to %s: %s\033[0m\n", ServerAddr, err)
				}
			}
		}
	}
}

func sendMessages(Servers map[string]net.Conn, timeStamps map[string]time.Time) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.HasPrefix(input, "grep ") {
			fmt.Printf("\033[1mRequesting Server grep...\n\033[0m")

			// Send the inputted text to all connected Servers
			for ServerAddr, ServerConn := range Servers {
				fmt.Printf("\033[34mSending data to %s\n\033[0m", ServerAddr) // Print the connection identifier
				// Mark time when message sent
				timeStamps[ServerAddr] = time.Now()
				_, err := ServerConn.Write([]byte("Client: execute_command " + input + "\n"))
				if err != nil {
					fmt.Printf("\033[1;31mError sending message to Server: %s\033[0m\n", err)
				}
			}
		}
	}
}

func main() {
	// Store Server addresses
	Servers := make(map[string]net.Conn)
	// Store timestamps for latency calculation
	timeStamps := make(map[string]time.Time)
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		fmt.Printf("\033[1;31mError listening: %s\033[0m\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Printf("\033[1;38;5;207mClient started, waiting for connections...\033[0m\n")

	// Start a new goroutine to handle user input for grep-all request
	go sendMessages(Servers, timeStamps)

	for {
		// Accept incoming connections from Servers
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("\033[1;31mError accepting connection: %s\033[0m\n", err)
			continue
		}
		go handleConnection(conn, Servers, timeStamps) // Start a new goroutine to handle the connection
	}
}
