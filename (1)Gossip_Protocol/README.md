# Gossip Style Heartbeating

## Introduction
The program consists of a basic client-server communication system implemented in Golang. The client listens for incoming TCP connections from servers using ‘net.listen’ and each incoming client connection is set as a new goroutine using ‘listener.Accept()’ and ‘go handleConnection(conn, Servers, timeStamps)’. The handleConnection manages communication with each server by reading server data using ‘conn.Read’ and removing servers if a disconnect is detected. In addition, the server prompts the user to enter the client address and then proceeds to connect with it if valid. The server also establishes a TCP connection to the client using ‘net.Dial’. There are two goroutines which are ‘receiveMessages’ and ‘sendMessages’. Through those features, it allows the client to send commands (e.g. grep) to all servers and receive messages from them.

## Unit tests
There are three main tests, which are TestGenerateLog, TestClientWithMockServer, TestGrepQuery. 
- TestGenerateLog tests whether log files are generated as expected. It creates a temporary directory, generates a test log file with a specified number of entries, and checks if the log file was created successfully.
- TestClientWithMockServer tests if the server can connect to the client correctly. It starts a mock client by calling the startMockClient function and connects a server to it. Then, the client sends a message to check if the server responds with the expected output.
- TestGrepQuery1, TestGrepQuery2, TestGrepQuery3 test the functionality of log file queries using the ‘grep’ command. Each test specifies a log file path, a search string, and the expected output. Then, it executes a ‘grep’ command and compares the output with the expected result to ensure that the search functionality is working properly.

## Usage

### VM Set-Up Instructions... From Scratch

1. Go to `https://vc.cs.illinois.edu/ui` and switch on your VM's
2. ssh into the desired vm (do this in vs code - it will be easier that way). For our group, the vm will be of some form `user@fa23-cs425-73XX.cs.illinois.edu` where `user` is your NetID and `XX` is a number from `01` to `10`.
    - Open VSCode and hit the bottom-left most button
    - Select "Connect to Host... Remote-SSH"
    - Enter `user@fa23-cs425-73XX.cs.illinois.edu` replacing `user` for your NetID and `XX` for the VM number
    - Enter your password and you should be ssh'ed into the VM
2. You will also need to update/install golang (The machine is Fedora Linux)
```bash
sudo yum install golang
```
3. Open the terminal within the VM and clone the git repository
```bash
git clone https://gitlab.engr.illinois.edu/kl35/cs425_mp1.git
```
4. Open the CS425_MP1 project directory in the IDE
5. Within the `go.mod` file, change `go 1.21.0` to `go 1.23`
6. You should now be able to run `server/server.go` and `client/client.go`
7. Copy the corresponding vm file (e.g. `vmX.log`) to the directory, `(root) /log`
    - When grepping from the server, use `grep [options] [pattern] /log` since each machine is guarunteed to have the `/log` directory with exactly one log file called `vmX.log`.

## Running the machine as a server

1. Change directory into `server/`
2. Run `go run .`
3. Follow the prompts
    - Enter the server address and port number (e.g. `fa23-cs425-7301.cs.illinois.edu:9000`)
    - Indicate if you want to create a new log file or use an existing one
        - `new` -> Enter the name of the log file to create (entering "machine.1" will create `log/machine.1.log`)
        - `existing` -> Enter the location and name of an existing log file to use

## Running the machine as a client

1. Change directory into `client/`
2. Run `go run .`
3. The server should be up and running
