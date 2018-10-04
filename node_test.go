package laughing_fortnight

import (
  "testing"
  "time"
)

func TestStartNode(t *testing.T){
  var err error
  var node_ Node = NewNode("", "")
  var errc chan error = make(chan error, 0)

  // Close after n time
  go func() {
    <-time.After(20 * time.Millisecond)
    if err = node_.Close(); err != nil {
      t.Error(err)
    }
  }()

  go func() {
    node_.Start(errc)
  }()

  if err = <-errc; err != nil {
    t.Error(err)
  }
}

func TestCloseNode(t *testing.T){
  var err error
  var node_ Node = NewNode("", "")

  if err = node_.Close(); err != nil {
    t.Error(err)
  }
}
