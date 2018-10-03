package laughing_fortnight

import (
  "testing"
  "time"
)

func TestServerStart(t *testing.T){
  var err error
  var srv Server = NewServer("127.0.0.1", 8000)

  if srv == nil {
    t.Error("Error creating a server")
  }

  // Close after n time
  go func() {
    <-time.After(20 * time.Millisecond)
    srv.Close(nil)
  }()

  if err = srv.Start(); err != nil {
    t.Error(err)
  }
}
