package laughing_fortnight

import (
  "errors"
  "io"
  "fmt"
  "log"
  "net"
  "os"
  "os/signal"
  "sync"
  "syscall"
  "time"
)

type Server interface {
  // Start starts the server to accept incoming connections, announce and
  //  publish messages.
  Start() error

  // addClient appends a connection to the server
  addClient(conn net.Conn)

  // removeClient removes a client from the connection list
  removeClient(conn net.Conn)

  // replyMessage echos the message to the sender
  replyMessage(client *edge, message []byte)

  // broadcastMessage broadcasts a message to all the clients
  broadcastMessage(client *edge, message []byte)

  // Close sends a signal to the server, to exit gracefully.
  //  The server can announce to the peers that it going to shutdown in case
  //  that a custom message struct is delivered.
  Close(err error)
}

// server holds the TCP server
type server struct {
  conns []*edge
  done chan error
  ip net.IP
  ln net.Listener // Listen to other clients
  node_ Node // Listen to other peers
  peers []Node
  port uint16
  *sync.RWMutex
}

// edge represents a TCP connection to the server
type edge struct {
  conn net.Conn
  id int
  joinedAt int64
}

// StartServer starts a server
func StartServer(ip string, port uint16){
  var err error
  var srv = NewServer(ip, port)
  if err = srv.Start(); err != nil {
    log.Fatal(err)
  }
}

// NewServer initialize a new empty server
func NewServer(ip string, port uint16) (s Server) {
  var nodeStatus NodeStatus

  // The master always starts on port 8888, but if this port is not
  //  available, there might be another master.
  if port == 8888 {
    nodeStatus = MASTER
  } else {
    nodeStatus = CANDIDATE
  }

  return &server{
    conns: make([]*edge, 0),
    ip: []byte(ip),
    node_: NewNode(nodeStatus),
    port: port,
    RWMutex: new(sync.RWMutex),
  }
}

// Start starts the server to accept incoming connections, announce and
//  publish messages.
func (s *server) Start() (err error){
  var ch chan os.Signal = make(chan os.Signal)
  var conn net.Conn
  var addr string = fmt.Sprintf("%s:%d", string(s.ip), s.port)

  s.ln, err = net.Listen("tcp", addr)
  if err != nil {
    return errors.New("Error listening to " + addr)
  }

  // Exit gracefully
  go func() {
    signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
    <-ch
    s.Close(errors.New("Terminated by user"))
  }()

  // Listen to the exit signal
  go func() {
    // Block and wait for a signal to exit the program
    s.Close(<-s.done) // It will wait for the channel before executing
    log.Println("\n\nSending exit notificaiton to all clients\n")
  }()

  // Discover and accept connections from other nodes
  go s.node_.Start()

  // Accept connections from clients
  log.Println("Listening on", addr)
  for {
    conn, err = s.ln.Accept()
    if err != nil {
      log.Println(err)
      return errors.New(fmt.Sprintf("Error listening on port", s.port))
    }

    go s.addClient(conn)
  }

  return err
}

// addClient appends a connection to the server
func (s *server) addClient(conn net.Conn) {
  var newEdge *edge = &edge{
    id: len(s.conns)+1,
    conn: conn,
    joinedAt: time.Now().Unix(),
  }

  s.Lock()
  s.conns = append(s.conns, newEdge)
  s.Unlock()
  defer s.removeClient(conn)

  var err error
  var buf []byte

  L:
  for {
    buf = make([]byte, 12000)
    _, err = conn.Read(buf)
    switch err {
    case io.EOF:
      log.Println("User", newEdge.id, "/", len(s.conns), "has left")
      break L
    case nil:
      break
    default:
      log.Fatal(err)
    }

    s.replyMessage(newEdge, buf)
    s.broadcastMessage(newEdge, buf)
  }
}

// removeClient removes a client from the connection list
func (s *server) removeClient(conn net.Conn) {
  for i, edge_ := range s.conns {
    if (edge_.conn == conn){
      s.Lock()
      // Replace the last item with this edge
      s.conns[i] = s.conns[len(s.conns)-1]
      s.conns = s.conns[:len(s.conns)-1]
      s.Unlock()
    }
  }
}

// replyMessage echos the message to the sender
func (s *server) replyMessage(client *edge, message []byte) {
  client.conn.Write(message)
}

// broadcastMessage broadcasts a message to all the clients
func (s *server) broadcastMessage(client *edge, message []byte){
  for _, edge_ := range s.conns {
    if edge_.conn == client.conn {
      // Optional - do not broadcast to the sender
      continue
    }
    s.replyMessage(edge_, message)
  }
}


// Close sends a signal to the server, to exit gracefully.
//  The server can announce to the peers that it going to shutdown in case
//  that a custom message struct is delivered.
func (s *server) Close(err error) {
  log.Println("Closing the server.\n", err)

  // Announce closing to all the clients here
  // ..
  s.broadcastMessage(&edge{}, []byte("\n\n*** SERVER IS SHUTTING DOWN ***\n"))

  if err = s.node_.Close(); err != nil {
    log.Fatal(err)
  }
  // Close the network connection
  err = s.ln.Close()
  log.Println("Closing the network connection.\n", err)

  // Exit the program
  os.Exit(0)
}
