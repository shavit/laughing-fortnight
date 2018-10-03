# TCP Server

TCP server written in Go.

## Usage
1. Download [go](https://golang.org/doc/install#testing)
2. Download the program `go get git github.com/shavit/laughing_fortnight`
3. Build `go build github.com/shavit/laughing_fortnight/cmd/`
4. Run `./cmd`

### Server
The server echos back messages. To start the server:
```
`./cmd server`
```

This will start a TCP server that will accept connections on 2 ports, from both CLI clients and other nodes.

## Client
The client that included in this program will try to print out the messages as string, but you can comment out the print command and rebuild the client.

To start the client:
```
$ ./cmd client
```

## Nodes
The server will hold a list of the other nodes in the network. Each node will try to connect first to the master node, in a known address.

A new node connection will make a request for all the available nodes in the network.

To run as a node:
```
`./cmd -p PORT server`
```

The node will ignore the `port` and `host` arguments if there is no master.

### Reconnect as a master
When the server dies, the oldest node or the last node, will take the master port and restart the connection.

When the slave becomes a master it will only establish a TCP connection to the other nodes. The client will lose the connection.

### Election
In this example the nodes do not communicate with each other, and there is no scoring system or health checks on each node.
