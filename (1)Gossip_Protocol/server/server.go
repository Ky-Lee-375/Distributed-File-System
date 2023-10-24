package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// receiveMessages reads and prints messages received from the server.
func receiveMessages(conn net.Conn, filePath string) {
	// Confirms location of log file
	clearTerminal()
	// fmt.Printf("\x1b[1;34mLog file has been set as %s\x1b[0m\n", filePath)
	fmt.Printf("\033[1;33m%s\033[0m\n", "------------------------------------------------")

	// Watch for incoming messages
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("\033[1;31mError receiving message: %s\033[0m\n", err)
			break
		}

		// Check if the received message requests a command execution
		if strings.HasPrefix(message, "Client: execute_command ") {
			command := strings.TrimRight(strings.TrimPrefix(message, "Client: execute_command "), "\n")
			fmt.Printf("%s\n", command)

			// Handle "grep " command
			if strings.HasPrefix(command, "grep ") {
				output, err := executeCommand(command)
				if err != nil {
					fmt.Print(err)
				} else {
					// Send output to server
					_, err = conn.Write([]byte("Server: grep_output\n" + output))
					if err != nil {
						fmt.Printf("\033[1;31mError sending output to server: %s\033[0m\n", err)
					}
				}
			} else {
				// Send output to server
				_, err = conn.Write([]byte("Server: execution blocked use \"grep\"\n"))
			}
		} 
	}
}

// Send messages to the server
func sendMessages(conn net.Conn) {
	// Read user input and send it to the server.
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		_, err := conn.Write([]byte(message + "\n"))
		if err != nil {
			fmt.Printf("\033[1;31mError sending message: %s\033[0m\n", err)
			return
		}
	}
}

func main() {
	// Prompt the user to enter the server address
	fmt.Printf("\x1b[1;33mEnter the client to connect to: \x1b[0m\n")
	connectionScanner := bufio.NewScanner(os.Stdin)
	connectionScanner.Scan()
	server := connectionScanner.Text()

	// Prompt the user for the option (new/existing) to define a log file
	var filePath string

	/**
	for {
		fmt.Printf("\x1b[1;33mGenerate new file or use existing (new/existing): \x1b[0m\n")
		optionScanner := bufio.NewScanner(os.Stdin)
		optionScanner.Scan()
		option := optionScanner.Text()

		if option == "new" {
			// Prompt the user to enter the name of a new Log file to generate
			fmt.Printf("\x1b[1;33mEnter the name of a new Log file to generate: \x1b[0m")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			fileName := scanner.Text()
			directory := "log"
			filePath = directory + "/" + fileName + ".log"
			generateLog(directory, fileName, 75)
			break
		} else if option == "existing" {
			// Prompt the user to enter the path of an existing log file
			fmt.Printf("\x1b[1;33mEnter the path of an existing Log file: \x1b[0m")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			filePath = scanner.Text()

			// Check if the file exists
			fileInfo, err := os.Stat(filePath)
			if err == nil {
				// File exists
				if !fileInfo.IsDir() && strings.HasSuffix(filePath, ".log") {
					// File is not a directory and ends with ".log"
					break
				} else {
					// File path does not point to a valid log file, show an error message
					fmt.Println("\x1b[1;31mFile is not a valid log file\x1b[0m")
				}
			} else if os.IsNotExist(err) {
				// File doesn't exist, show an error message
				fmt.Println("\x1b[1;31mFile could not be resolved\x1b[0m")
			}
		} else {
			// Invalid option, show an error message and repeat the loop
			fmt.Println("\x1b[1;31mInvalid option. Please enter 'new' or 'existing'\x1b[0m")
		}
	}
	**/

	// Connect to the server running on localhost at port 9000.
	conn, err := net.Dial("tcp", server)
	if err != nil {
		fmt.Printf("\033[1;31mError connecting to server: %s\033[0m\n", err)
		return
	}
	defer conn.Close()

	// Start a goroutine to receive and process messages from the server.
	go receiveMessages(conn, filePath)

	// Start a goroutine to send messages to the server.
	go sendMessages(conn)

	// Read user input and send it to the server.
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		message := scanner.Text()
		_, err := conn.Write([]byte(message + "\n"))
		if err != nil {
			fmt.Printf("\033[1;31mError sending message: %s\033[0m\n", err)
			return
		}
	}
}
