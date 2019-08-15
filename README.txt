Usage:

import "fmt"
import "mikrotik"

const DEV_TIMEOUT = 10
const DEV_USER="admin"
const DEV_PASS=""
const DEV_PORT="8728"

func main() {

  // send "stop" command down to channel, to interrupt any data transfers
  var ctrl_ch chan string
  ctrl_ch= make(chan string, 1)

  dev := mikrotik.Init("dev.address", DEV_PORT, DEV_TIMEOUT*time.Second, ctrl_ch)
  defer dev.Close()

  // Turning debugging output on
  //dev.Debug=true

  // Command responses are returned as sentences
  var snt *mikrotik.MkSentence

  var err error
  var err_str string="No error"


  err = dev.Connect(dev_user, dev_pass)
  if(err != nil) {
    err_str = "ERROR: connect error: "+err.Error()
    fmt.Printf("%s\n", err_str)
    os.Exit(1)
  }

  commands := []string{"/interface/print", "=.proplist=name,type", "?type=ether"}

  err = dev.Send(commands...)
  if(err != nil) {
    err_str = "ERROR: comm error: "+err.Error()
    fmt.Printf("%s\n", err_str)
    os.Exit(1)
  }
  snt,err = dev.ReadSentence()
  for err == nil && snt.Answer == "!re" {
    snt.Dump()

    snt,err = dev.ReadSentence()
  }

  if(err != nil) {
    err_str = "ERROR: comm error: "+err.Error()
    fmt.Printf("%s\n", err_str)
    os.Exit(1)
  }

  if(snt.Answer != "!done") {
    err_str = "ERROR: no !done in last reply"
    fmt.Printf("%s\n", err_str)
    os.Exit(1)
  }
}

If command returns only one "!re" answer, you may use:

  snt, err := dev.SendReadDone(commands...)

  it will fail if there is no or more than one !re reply or there is no !done int the end

