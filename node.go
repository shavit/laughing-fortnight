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
  MASTER
	SLAVE
	CANDIDATE
  MASTER_PORT = 8000
  MASTER_ADDRESS = "127.0.0.1:8000"
  ELECTION_KEY = "leader_0"
)

// Node represents a server in a cluster.
type Node interface {

  // Start starts the TCP server
  Start() (err error)

  // register sends the node address to etcd
  register() (err error)

  // listNodes list and prints out all the registered nodes on etcd
  listNodes() (err error)

  // campaign start an election for a leader
  campaign() (err error)

  // handleMessage handle incoming messages
  handleMessage(conn net.Conn, message []byte) (err error)

  // findNewMaster lookup for a new master server
  //findNewMaster() (err error)

  // Close closes the TCP connection
  Close() (err error)

}

type node struct {
  addr string
  conn net.Conn
  etcdCtl *etcd_client.Client
  etcdEndpoint string
  id uint64
  ln net.Listener
  status NodeStatus
  masterAddr string
  peers map[string]net.Conn
  rNode Node
  done chan struct{}
  *sync.RWMutex
}

func NewNode(status NodeStatus, endpoint string) Node {
  var n *node = &node{
    done: make(chan struct{}, 1),
    etcdEndpoint: endpoint,
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

    n.ln, err = net.Listen("tcp", n.addr)
    if err == nil {
      log.Println("Node connected on", n.addr, n.status)
      break
    }

    port += 1
    n.addr = fmt.Sprintf("127.0.0.1:%d", port)
  }

  go n.register()

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

// register sends the node address to etcd
func (n *node) register() (err error) {
  var config etcd_client.Config = etcd_client.Config{Endpoints: []string{n.etcdEndpoint}}
  n.etcdCtl, err = etcd_client.New(config)
  if err != nil {
    return
  }
  defer n.etcdCtl.Close()

  go n.listNodes()
  for {
    if err = n.campaign(); err != nil {
      return err
    }
  }

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
      log.Println("[etcd] [self] Node", leaderAddr)
    } else {
      log.Println("[etcd] Node", leaderAddr)
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
  log.Println("[etcd] New leader", leaderAddr, "Restarting")
  if leaderAddr == n.addr {
    n.status = MASTER
    return n.Start()
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
  n.done<- struct{}{}

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
