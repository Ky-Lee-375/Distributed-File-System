// client/main_test.go

package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

////////////////////////////////////////////////////////////////
////					HELPER FUNCTIONS					////
////////////////////////////////////////////////////////////////

// Mock Client function
func startMockClient(t *testing.T, addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Error starting mock Client: %s", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			t.Fatalf("Error accepting connection: %s", err)
		}
		go handleMockServer(conn)
	}
}

// Mock server handler
func handleMockServer(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	message, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading message from server: %s\n", err)
		return
	}

	response := "Client: execute_command " + message
	_, err = conn.Write([]byte(response))
	if err != nil {
		fmt.Printf("Error sending response to server: %s\n", err)
	}
}

////////////////////////////////////////////////////////////
////					TEST SUITE						////
////////////////////////////////////////////////////////////

// Tests if log files are being generated as expected
func TestGenerateLog(t *testing.T) {
	tempDir := t.TempDir()
	filename := "test_log"
	// Change numEntries for more testing
	numEntries := 10

	generateLog(tempDir, filename, numEntries)

	// Check if the log file was created
	logFilePath := filepath.Join(tempDir, fmt.Sprintf("%s.log", filename))
	_, err := os.Stat(logFilePath)
	if os.IsNotExist(err) {
		t.Errorf("Expected log file to be created, but it wasn't")
	}
}

// Tests if clients can connect to the server correctly
func TestClientWithMockServer(t *testing.T) {
	// Start a mock server
	clientAddr := "localhost:9001"
	go startMockClient(t, clientAddr)

	// Wait for the mock server to start
	time.Sleep(100 * time.Millisecond)

	// Connect to the mock server
	conn, err := net.Dial("tcp", clientAddr)
	if err != nil {
		t.Fatalf("Error connecting to client: %s", err)
	}
	defer conn.Close()

	// Send a message to the mock client
	message := "Hello, mock client!"
	// fmt.Printf(message + "\n")
	_, err = conn.Write([]byte(message + "\n"))
	if err != nil {
		t.Fatalf("Error sending message to client: %s", err)
	}

	// Read response from the mock client
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	// fmt.Printf(response + "\n")
	if err != nil {
		t.Fatalf("Error reading response from client: %s", err)
	}

	// Expected response from the mock client
	expectedResponse := "Client: execute_command " + message
	// fmt.Printf(expectedResponse + "\n")
	// Compare the actual response with the expected response
	if strings.TrimSpace(response) != expectedResponse {
		t.Errorf("Unexpected response from client. Expected: %s, Got: %s", expectedResponse, response)
	}
}

// Test (1) if log file query using grep works as expected – func queryGrep(...)
func TestGrepQuery1(t *testing.T) {
	logFilePath := "./test/test1.i.log"
	searchString := "is"
	expectedOutput := "2:watermelon is the best fruit\n5:matcha is the best!\n6:Nemo is Dory's friend\n"

	cmd := "grep -n " + searchString + " " + logFilePath

	output, err := executeCommand(cmd)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if output != expectedOutput {
		t.Errorf("Expected output:\n%s\nBut got:\n%s", expectedOutput, output)
	}
}

// Test (2) if log file query using grep works as expected – func queryGrep(...)
func TestGrepQuery2(t *testing.T) {
	logFilePath := "test/test2.i.log"
	searchString := "roads"
	expectedOutput := "1:Two roads diverged in a yellow wood,\n21:Two roads diverged in a wood, and I—\n"

	cmd := "grep -n " + searchString + " " + logFilePath

	output, err := executeCommand(cmd)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if output != expectedOutput {
		t.Errorf("Expected output:\n%s\nBut got:\n%s", expectedOutput, output)
	}
}

// Test (3) if log file query using grep works as expected – func queryGrep(...)
func TestGrepQuery3(t *testing.T) {
	logFilePath := "test/test3.i.log"
	searchString := "ear"
	expectedOutput := "3:Bear this torch against the cold of the night\n4:Search your soul and re-awaken the undying light!\n28:Bear this torch against the cold of the night\n29:Search your soul and re-awaken the undying light!\n48:Bear this torch against the cold of the night\n58:Bear this torch against the cold of the night\n"

	cmd := "grep -n " + searchString + " " + logFilePath

	output, err := executeCommand(cmd)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if output != expectedOutput {
		t.Errorf("Expected output:\n%s\nBut got:\n%s", expectedOutput, output)
	}
}
