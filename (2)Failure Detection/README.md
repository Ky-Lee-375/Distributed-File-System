# Gossip-Based Failure Detection System

## Design
In our distributed group membership system, each node should maintain a membership list of all other nodes in the network using a map of NodeDetails structs (ID, IPAddress, Port, Version, Status, LogicalClock). Information will be spread via pull gossip where each node will choose k random nodes to pull every 2 seconds. The gossip may include information about new nodes that have joined, nodes that have faile/left, or those that are unresponsive.

When a node first starts up, it sends a join request to the introducer which multicasts the information of the new node to all other nodes in its own membership list. The introducer’s membership list is roughly maintained by pinging all of the nodes in its membership list and removing nodes that do not send back an acknowledgement response. This list only needs to be roughly maintained since information about each node in the network will be disseminated via gossip protocol as long as the introducer maintains a list of a subset of nodes in that network.

While the system is up, four go routines will be running on each machine. (1) pullMembership takes care of pulling from a random subset of nodes and requesting their membership list information on the other nodes in the network. (2)  listenForGossip listens on a specific port for UDP messages of the following types: “LEAVING” indicates the sender is voluntarily leaving the network, “GOSSIP_REQUEST” means the sender is requesting the membership list of the receiver node, “CHECK” is exclusively used by the introducer to check if nodes in its membership list are still alive, and “SUSPICION_CHANGE” is sent by a node to notify all other nodes to enable the suspicion mechanism. (3) watchForCommands reads the CLI for the following commands: “list_mem”, “list_self”, “enable_sus”, “disable_sus”, and “leave.” (4) incrementLogicalClock updates the local time at each node every second with 0 being the time at node startup.

Since there are common resources that multiple go routines may be trying to access simultaneously while the system is running, such as each node’s membership list (read and write may occur at any point), we used mutex locks to protect critical sections and prevent race conditions. For instance, the case may arise where we want to print the node’s membership list to terminal while a new node is being written to it. In this case, a mutex lock is needed to ensure that these operations are synchronized and the access to the shared resource is controlled.


## Running Introducer Node
Navigate to the Introducer directory: `"cd introducer"` <br>
Run Introducer: `go run main.go -ip="[ip]"`<br>

## Running Regular Nodes
Navigate to the node directory: `"cd node"` <br>
Run Node: `go run main.go -id="[node_id]" -ip="[ip_address]" -port=[port_number] -introducer_ip="[introducer_ip]"` <br>
(Replace <node_id>, <ip_address>, <port_number>, and <introducer_ip_address> with appropriate values.) <br>

When running locally: <br>
ex. `go run *.go -id="node-1" -ip="localhost" -port=9001 -introducer_ip="localhost"` <br>
ex. `go run *.go -id="node-2" -ip="localhost" -port=9002 -introducer_ip="localhost"` <br>
ex. `go run *.go -id="node-3" -ip="localhost" -port=9003 -introducer_ip="localhost"` <br>
ex. `go run *.go -id="node-5" -ip="localhost" -port=9005 -introducer_ip="localhost"` <br>

When running on VMs: <br>
1. `go run *.go -id="node-1" -ip="172.22.156.242" -port=9000 -introducer_ip=""`
2. `go run *.go -id="node-2" -ip="172.22.158.242" -port=9000 -introducer_ip=""`
3. `go run *.go -id="node-3" -ip="172.22.94.242" -port=9000 -introducer_ip=""`
4. `go run *.go -id="node-4" -ip="172.22.156.243" -port=9000 -introducer_ip=""`
5. `go run *.go -id="node-5" -ip="172.22.158.243" -port=9000 -introducer_ip=""`
6. `go run *.go -id="node-6" -ip="172.22.94.243" -port=9000 -introducer_ip=""`
7. `go run *.go -id="node-7" -ip="172.22.156.244" -port=9000 -introducer_ip=""`
8. `go run *.go -id="node-8" -ip="172.22.158.244" -port=9000 -introducer_ip=""`
9. `go run *.go -id="node-9" -ip="172.22.94.244" -port=9000 -introducer_ip=""`
10. `go run *.go -id="node-10" -ip="172.22.156.245" -port=9000 -introducer_ip=""`
## Interacting with Nodes
Use the following commands to interact with the nodes:

- list_mem: Print the current membership list.
- list_self: Print the ID of the current node.
- enable_sus: Enable suspicion mechanism.
- disable_sus: Disable suspicion mechanism.
- leave: Notify other nodes about leaving and terminate the program.