## System components of SDFS (Simple Distributed File System)

Leader/Master

- Serves as coordinator and manages `put` and `get` requests for files.
- For `put` requests it will choose one location to store the file based on a hash function and file metadata. The file will then get replicated to that location and its 3 other successors in the Leader's membership list
- For `get` requests clients will make requests to the leader, the leader will add it to a queue, and when a get request is processed, it will hash the file name, find the machine the file is supposed to be stored on, and retrieve it. If that machine is down, the leader will know to go check its 3 successors.
- In the queue, the leader cannot process more then 4 consecutive reads or 4 consecutive writes.

Client/Worker

- Will recieve a fetch request from the Leader to get a file from its local directory
- Store membership list of all other nodes. If the leader goes down, each node should use their membership list to elect a new leader.
- Be able to replicate data (file and metadata) in the event that a node goes down.

Membership List and Gossip

- Have a .txt file that stores the membership list at each machine. The best solution MP2 code will be modified to be abel to read and write from this file. Our MP3 code will be able to read from this membership list to know what nodes are alive at any given point. There needs to be a gateway to handle reads and writes using mutex locks since the .txt file is a shared resource.

## Functionalities

Leader will be the first node that joins, and will also be known as the introducer. When other nodes join through the introducer, it will also know who the leader is by checking the Host.isLeader boolean value within their membership lists. The clients will then know to route all requests to that leader node. If this node fails, the first to detect it will automatically elect a new leader, and pass that information around through gossip. However, if the introducer fails, it is not necessary to elect a new introducer, but no new nodes can join unless it comes back online.

Nodes will be ranked in order of seniority. This means if the goes down, all requests will be routed to the next most senior node. Additionally, the next most senior node should know that it has been elected as the new leader. For consistency, we will define senority as the longest continuously running node, so a node that returns to the network will be considered the least senior, since it has the shortest continuous run time.

When a new leader is appointed, it will need to replicate all of the files of the downed nodes. Choosing where to replicate each file will be done by 