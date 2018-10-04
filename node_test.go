package laughing_fortnight

import (
  "testing"
  "time"
)

func TestStartNode(t *testing.T){
  var err error
  var node_ Node = NewNode(MASTER, "")

  // Close after n time
  go func() {
    <-time.After(20 * time.Millisecond)
    if err = node_.Close(); err != nil {
      t.Error(err)
    }
  }()

  if err = node_.Start(); err != nil {
    t.Error(err)
  }
}

func TestCloseNode(t *testing.T){
  var err error
  var node_ Node = NewNode(SLAVE, "")

  if err = node_.Close(); err != nil {
    t.Error(err)
  }
}
