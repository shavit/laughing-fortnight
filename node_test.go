package laughing_fortnight

import (
  "testing"
  "time"
)

func TestStartNode(t *testing.T){
  var err error
  var node_ Node = NewNode(MASTER)

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

/*
func TestRegisterNode(t *testing.T){
  var err error
  var nodeMaster Node = NewNode(MASTER)
  var nodeSlave Node = NewNode(CANDIDATE)

  // Close after n time
  go func() {
    <-time.After(20 * time.Millisecond)
    if err = nodeSlave.Close(); err != nil {
      t.Error(err)
    }
    if err = nodeMaster.Close(); err != nil {
      t.Error(err)
    }
  }()

  if err = nodeMaster.Start(); err != nil {
    t.Error(err)
  }

  if err = nodeSlave.register(); err != nil {
    t.Error(err)
  }
}
*/

func TestCloseNode(t *testing.T){
  var err error
  var node_ Node = NewNode(SLAVE)

  if err = node_.Close(); err != nil {
    t.Error(err)
  }
}
