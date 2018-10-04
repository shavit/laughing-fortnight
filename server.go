package laughing_fortnight

import (
  "errors"
  "fmt"
  "io"
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

  // acceptClients opens a TCP socket for clients
  acceptClients() (err error)

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
  status NodeStatus
  *sync.RWMutex
}

// edge represents a TCP connection to the server
type edge struct {
  conn net.Conn
  id int
  joinedAt int64
}

// StartServer starts a server
func StartServer(ip string, port uint16, endpoint string){
  var err error
  var srv = NewServer(ip, port, endpoint)
  if err = srv.Start(); err != nil {
    log.Fatal(err)
  }
}

// NewServer initialize a new empty server
func NewServer(ip string, port uint16, endpoint string) (s Server) {
  return &server{
    conns: make([]*edge, 0),
    done: make(chan error, 1),
    ip: []byte(ip),
    node_: NewNode(ip, endpoint),
    port: port,
    RWMutex: new(sync.RWMutex),
  }
}

// Start starts the server to accept incoming connections, announce and
//  publish messages.
func (s *server) Start() (err error){
  // Exit gracefully
  var ch chan os.Signal = make(chan os.Signal)
  go signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

  // Discover and accept connections from other nodes
  go s.node_.Start(s.done)

  // Accept connections from clients
  var acch chan error = make(chan error, 1)
  go func() {
    if err = s.acceptClients(); err != nil {
      acch<- err
    }
  }()

  for {
    select {

    // When the node wants to restart the server as the master
    case err = <-s.done:
      log.Println("Server restarted by node", err)
      s.port = 8888
      return s.Start()

    // When the address is not available
    case err = <-acch:
      if s.port > 8800 && s.port < 10000 {
        s.port += 1
        time.Sleep(time.Duration(1 * time.Second))
        return s.Start()
      } else {
        s.Close(err)
        return err
      }

    // Shutdown the server
    case <-ch:
      s.Close(errors.New("Terminated by user"))
      return err
    }
  }

  return err
}

// acceptClients opens a TCP socket for clients
func (s *server) acceptClients() (err error){
  // The master always starts on port 8888, but if this port is not
  //  available, there might be another master.
  if s.port == 8888 {
    s.status = LEADER
  } else {
    s.status = FOLLOWER
  }

  var conn net.Conn
  var addr string = fmt.Sprintf("%s:%d", string(s.ip), s.port)
  s.ln, err = net.Listen("tcp", addr)
  if err != nil {
    return errors.New("Error listening on" + addr)
  }
  log.Println("Server listens to clients on", addr, s.status.String())

  for {
    conn, err = s.ln.Accept()
    if err != nil {
      log.Println(err)
      return errors.New(fmt.Sprintf("Error listening on port", s.port))
    }

    go s.addClient(conn)
  }
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
  s.broadcastMessage(&edge{}, []byte("\n\n*** SERVER IS SHUTTING DOWN ***\n"))

  if err = s.node_.Close(); err != nil {
    log.Fatal(err)
  }
  // Close the network connection
  if s.ln != nil {
    err = s.ln.Close()
    log.Println("Closing the network connection.\n", err)
  }

  // Exit the program
  os.Exit(0)
}
