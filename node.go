package laughing_fortnight

import (
  "errors"
  "fmt"
  "io"
  "log"
  "net"
  "sort"
  "sync"
  "time"
)

type NodeStatus byte
const (
  _ NodeStatus = iota
  MASTER
	SLAVE
	CANDIDATE
  MASTER_PORT = 8000
  MASTER_ADDRESS = "127.0.0.1:8000"
)

// Node represents a server in a cluster.
type Node interface {

  // Start starts the TCP server
  Start() (err error)

  // register sends the node address to the master
  register() (err error)

  // handleMessage handle incoming messages
  handleMessage(conn net.Conn, message []byte) (err error)

  // findNewMaster lookup for a new master server
  findNewMaster() (err error)

  // Close closes the TCP connection
  Close() (err error)

}

type node struct {
  addr string
  conn net.Conn
  id int64
  ln net.Listener
  status NodeStatus
  masterAddr string
  peers map[string]net.Conn
  rNode Node
  *sync.RWMutex
}

func NewNode(status NodeStatus) Node {
  var n *node = &node{
    id: time.Now().Unix(),
    status: status,
    peers: make(map[string]net.Conn, 0),
    RWMutex: new(sync.RWMutex),
  }

  return n
}

// Start starts the TCP server
func (n *node) Start() (err error) {
  var conn net.Conn
  var port int = 9001
  if n.status == MASTER {
    n.addr = MASTER_ADDRESS
    port = MASTER_PORT
  } else {
    n.status = SLAVE
    n.addr = fmt.Sprintf("127.0.0.1:%d", port)
  }

  // Scan the available ports in case the peers are on the same machine
  for {
    if port > 9200 {
      return errors.New("Error listening to " + n.addr)
    }

    log.Println("Node connected on", n.addr, n.status)
    n.ln, err = net.Listen("tcp", n.addr)
    if err == nil {
      break
    }

    port += 1
    n.addr = fmt.Sprintf("127.0.0.1:%d", port)
  }

  if n.status != MASTER {
    go n.register()
  }

  // Accept connections from other nodes
  for {
    conn, err = n.ln.Accept()
    if err != nil {
      log.Println(err)
      return errors.New(fmt.Sprintf("Error listening on %v", n.addr))
    }

    // Unlimited nodes
    go n.addNode(conn)
  }

  return err
}

// register sends the node address to the master
func (n *node) register() (err error) {
  if n.masterAddr == "" {
    n.masterAddr = MASTER_ADDRESS
  }
  n.conn, err = net.Dial("tcp", n.masterAddr)
  if err != nil {
    log.Println(err)
    return n.findNewMaster()
  }

  err = SendMessage(n.conn, &nodeMessage{
    Event: "GET_NODES",
    Body: []byte(n.addr),
  })
  if err != nil {
    return err
  }

  // Listen to messages
  for {
    buf := make([]byte, 64000)
    _, err = n.conn.Read(buf)
    if err != nil {
      log.Println("Connection closed by the server.")
      return n.findNewMaster()
    }

    n.handleMessage(n.conn, buf)

    buf = nil
  }

  return err
}

func (n *node) addNode(conn net.Conn) {
  var addr = conn.RemoteAddr().String()

  n.Lock()
  n.peers[addr] = conn
  if n.rNode == nil {
    n.rNode = &node{
      conn: conn,
      addr: addr,
      status: SLAVE,
    }
  }
  n.Unlock()
  defer n.removeNode(conn)

  var err error
  var buf []byte

  L:
  for {
    buf = make([]byte, 12000)
    _, err = conn.Read(buf)
    switch err {
    case io.EOF:
      log.Println("Node", addr, "has left")
      break L
    case nil:
      break
    default:
      log.Println("Node", addr, "has disconnected")
      break L
    }

    n.handleMessage(conn, buf)
  }
}

func (n *node) removeNode(conn net.Conn) {
  n.Lock()
  for a, c := range n.peers {
    if c == conn {
      delete(n.peers, a)
      break
    }
  }
  n.Unlock()
}

// findNewMaster lookup for a new master server
func (n *node) findNewMaster() (err error) {
  if n.masterAddr != "" {
    delete(n.peers, n.masterAddr)
  }

  log.Println("FindNewMaster", len(n.peers))
  if len(n.peers) <= 1 {
    // Restart
    n.status = MASTER
    if err = n.Close(); err != nil {
      return err
    }
    log.Println("Restarting as a master node", len(n.peers))
    return n.Start()
  }

  // Sort the address and vote for the first node
  var addresses []string = make([]string, len(n.peers))
  for item, _ := range n.peers {
    addresses = append(addresses, item)
  }
  sort.Strings(addresses)
  n.masterAddr = addresses[0]
  if err = n.Close(); err != nil {
    return err
  }
  if n.masterAddr == n.addr {
    n.status = MASTER
    log.Println("Restarting as a master node", len(n.peers))
  } else {
    log.Println("Restarting as a slave node", len(n.peers))
  }

  return n.Start()
}

// handleMessage handle incoming messages
func (n *node) handleMessage(conn net.Conn, message []byte) (err error) {
  msg, err := ParseMessage(message)
  if err != nil {
    return err
  }

  switch msg.Event {
  case "ADD_NODE":
    // Add a node if not exists
    n.peers[string(msg.Body)] = nil
    log.Println("Add new node", len(n.peers))
    break
  case "NODE_LEFT":
    delete(n.peers, string(msg.Body))
    break
  case "GET_NODES":
    // Send a direct message with all the nodes
    for nAddr, _ := range n.peers {
      SendMessage(conn, &nodeMessage{
        Event: "ADD_NODE",
        Body: []byte(nAddr),
      })
    }
    break
  }

  return err
}

// Close closes the TCP connection
func (n *node) Close() (err error) {
  SendMessage(n.conn, &nodeMessage{
    Event: "NODE_LEFT",
    Body: []byte(n.addr),
  })
  if n.ln != nil {
    err = n.ln.Close()
  }
  n.ln = nil

  return err
}

type nodeMessage struct {
  Event string
  Body []byte
}

func ParseMessage(msg []byte) (message *nodeMessage, err error) {
  var n int = len(msg)

  for i := range msg {
    if string(msg[i:i+1]) == ":" {
      return &nodeMessage{
        Event: string(msg[0:i]),
        Body: msg[i+1:n],
      }, err
    }
  }

  return message, errors.New("Invalid message format")
}

func SendMessage(conn net.Conn, msg *nodeMessage) (err error) {
  if conn == nil {
    return errors.New("Cannot write a message to a nil connection")
  }
  var message = fmt.Sprintf("%s:%s\n", msg.Event, msg.Body)
  _, err = io.WriteString(conn, message)
  return err
}
