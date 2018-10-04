package laughing_fortnight

import (
  "context"
  "errors"
  "fmt"
  "io"
  "log"
  "net"
  "sync"

  etcd_client "go.etcd.io/etcd/clientv3"
  etcd_concurrency "go.etcd.io/etcd/clientv3/concurrency"
)

type NodeStatus byte
const (
  _ NodeStatus = iota
  LEADER
	FOLLOWER
  ELECTION_KEY = "leader_0"
)

func (n NodeStatus) String() string {
  switch n {
  case LEADER:
    return "Leader"
  case FOLLOWER:
    return "Follower"
  default:
    return ""
  }
}

// Node represents a server in a cluster.
type Node interface {

  // Start starts the TCP server
  Start(errc chan error)

  // register sends the node address to etcd
  register() (err error)

  // listNodes list and prints out all the registered nodes on etcd
  listNodes() (err error)

  // campaign start an election for a leader
  campaign() (err error)

  // handleMessage handle incoming messages
  handleMessage(conn net.Conn, message []byte) (err error)

  // Close closes the TCP connection
  Close() (err error)

}

type node struct {
  addr string
  conn net.Conn
  etcdCtl *etcd_client.Client
  etcdEndpoint string
  id uint64
  ip string
  ln net.Listener
  masterAddr string
  peers map[string]net.Conn
  rNode Node
  *sync.RWMutex
}

func NewNode(ip string, endpoint string) Node {
  var n *node = &node{
    etcdEndpoint: endpoint,
    ip: ip,
    peers: make(map[string]net.Conn, 0),
    RWMutex: new(sync.RWMutex),
  }

  return n
}

// Start starts the TCP server
func (n *node) Start(errc chan error) {
  var err error
  var conn net.Conn
  var port int = 9001
  n.addr = fmt.Sprintf("%s:%d", n.ip, port)

  // Scan the available ports in case the peers are on the same machine
  for {
    n.addr = fmt.Sprintf("127.0.0.1:%d", port)
    if port > 10000 {
      errc<- errors.New("Error listening to " + n.addr)
      return
    }

    n.ln, err = net.Listen("tcp", n.addr)
    if err == nil {
      log.Println("Node connected", n.addr)
      break
    }

    port += 1
  }

  go n.register()

  // Accept connections from other nodes
  for {
    if n.ln == nil {
      break
    }
    conn, err = n.ln.Accept()
    if err != nil {
      log.Println(err)
      errc<- errors.New(fmt.Sprintf("Error listening on %v", n.addr))
      return
    }

    // Unlimited nodes
    go n.addNode(conn)
  }
}

// register sends the node address to etcd
func (n *node) register() (err error) {
  var config etcd_client.Config = etcd_client.Config{Endpoints: []string{n.etcdEndpoint}}
  n.etcdCtl, err = etcd_client.New(config)
  if err != nil {
    return
  }
  defer n.etcdCtl.Close()

  go n.listNodes()
  n.campaign()

  return err
}

// listNodes list and prints out all the registered nodes on etcd
func (n *node) listNodes() (err error) {
  session, err := etcd_concurrency.NewSession(n.etcdCtl)
  if err != nil {
    return
  }
  defer session.Close()

  election := etcd_concurrency.NewElection(session, ELECTION_KEY)
  ctx, cancel := context.WithCancel(context.TODO())
  defer cancel()

  for resp := range election.Observe(ctx) {
    leaderAddr := string(resp.Kvs[0].Value)
    if n.addr == leaderAddr {
      log.Println("[etcd][self]", leaderAddr)
    } else {
      //log.Println("[etcd]", leaderAddr)
    }

  }

  return err
}

// campaign start an election for a leader
func (n *node) campaign() (err error) {
  session, err := etcd_concurrency.NewSession(n.etcdCtl)
  if err != nil {
    return err
  }
  defer session.Close()
  n.id = uint64(session.Lease())

  election := etcd_concurrency.NewElection(session, ELECTION_KEY)
  ctx, cancel := context.WithCancel(context.TODO())
  defer cancel()

  if err := election.Campaign(ctx, n.addr); err != nil {
    return err
  }

  resp, err := n.etcdCtl.Get(ctx, election.Key())
  if err != nil {
    return err
  }
  leaderAddr := string(resp.Kvs[0].Value)
  log.Println("[etcd] New leader", leaderAddr)
  if leaderAddr == n.addr {
    go election.Resign(context.TODO())
    return n.Close()
  }

  select {
  case <-session.Done():
    return errors.New("Election interruped")
  }

  return election.Resign(context.TODO())
}

func (n *node) addNode(conn net.Conn) {
  var addr = conn.RemoteAddr().String()

  n.Lock()
  n.peers[addr] = conn
  if n.rNode == nil {
    n.rNode = &node{
      conn: conn,
      addr: addr,
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
  log.Println("Node closed", n.addr)

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
