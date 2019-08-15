package mikrotik

import "time"
import "errors"
import "net"
import "fmt"
import "strings"
import "encoding/hex"
import "crypto/md5"

type MkDev struct {
  conn net.Conn
  Connected bool
  ip string
  port string
  timeout time.Duration
  stop_ch chan string
  Debug bool
  Bytes_in uint64
  Bytes_out uint64
  Bytes_since time.Time
}

type MkSentence struct {
  Answer string
  Attrs map[string]string
}

func Init(ip string, port string, timeout time.Duration, stop_ch chan string) (*MkDev) {
  dev :=new(MkDev)
  dev.Connected=false
  dev.ip=ip
  dev.port=port
  dev.timeout=timeout
  dev.stop_ch=stop_ch
  dev.Debug=false
  dev.Bytes_in=0
  dev.Bytes_out=0
  dev.Bytes_since=time.Now()
  return dev
}

func (dev *MkDev) Close() {
  if(dev.conn != nil) {
    if(dev.Debug) { println("Closing connection") }
    dev.conn.Close()
    dev.Connected=false
    dev.conn=nil
  }
  return
}

func (dev *MkDev) Send(commands...string) (error) {
  for _,cmd := range commands {
    if(dev.Debug) { println(">", cmd) }
    var to_send uint32= uint32(len(cmd))

    var bytes []byte

    if(to_send <= 0x7F) {
      bytes=append(bytes, byte(to_send))
    } else if(to_send <= 0x3FFF) {
      bytes=append(bytes, byte((to_send >> 8) & 0xFF), byte(to_send & 0xFF))
    } else if (to_send < 0x200000) {
      bytes=append(bytes, byte((to_send >> 16) & 0xFF), byte((to_send >> 8) & 0xFF),  byte(to_send & 0xFF))
    } else if (to_send < 0x10000000) {
      bytes=append(bytes, byte((to_send >> 24) & 0xFF), byte((to_send >> 16) & 0xFF), byte((to_send >> 8) & 0xFF),  byte(to_send & 0xFF))
    } else {
      bytes=append(bytes, 0xF0, byte((to_send >> 24) & 0xFF), byte((to_send >> 16) & 0xFF), byte((to_send >> 8) & 0xFF),  byte(to_send & 0xFF))
    };

    err := dev.conn.SetDeadline(time.Now().Add(dev.timeout))
    if(err != nil) { dev.Close(); return err }
    sent, err := dev.conn.Write(bytes)
    if(err != nil) { dev.Close(); return err }
    dev.Bytes_out += uint64(sent)
    if(sent != len(bytes)) { dev.Close(); return errors.New("short write") }

    err = dev.conn.SetDeadline(time.Now().Add(dev.timeout))
    if(err != nil) { dev.Close(); return err }
    sent, err = dev.conn.Write([]byte(cmd))
    if(err != nil) { dev.Close(); return err }
    dev.Bytes_out += uint64(sent)
    if(sent != len(cmd)) { dev.Close(); return errors.New("short write") }
    select {
      case cmd := <-dev.stop_ch:
        if(cmd == "stop") {
          dev.Close();
          return errors.New("exit signalled")
        }
      default:
        //do nothing
    }
  }
  err := dev.conn.SetDeadline(time.Now().Add(dev.timeout))
  if(err != nil) { dev.Close(); return err }
  sent, err := dev.conn.Write([]byte{0})
  if(err != nil) { dev.Close(); return err }
  dev.Bytes_out += uint64(sent)
  if(sent != 1) { dev.Close(); return errors.New("short write") }
  return nil
}

func (dev *MkDev) readByte() (byte, error) {
  read_buff := make([]byte, 1)
  err := dev.conn.SetDeadline(time.Now().Add(dev.timeout))
  if(err != nil) { dev.Close(); return 0,err }
  read, err := dev.conn.Read(read_buff)
  if(err != nil) { dev.Close(); return 0,err }
  dev.Bytes_in += uint64(read)
  if(read == 0) { dev.Close(); return 0,errors.New("Short read") }
  if(read != 1) { dev.Close(); return 0,errors.New(fmt.Sprintf("Too much read: %d, expected: %d", read, 1)) }
  select {
    case cmd := <-dev.stop_ch:
      if(cmd == "stop") {
        dev.Close();
        return 0,errors.New("exit signalled")
      }
    default:
      //do nothing
  }
  return read_buff[0], nil
}


func (dev *MkDev) readLine() (string, error) {
  var b byte
  var err error

  b, err = dev.readByte()
  if(err != nil) { dev.Close(); return "",err }

  var length uint32=uint32(b)
  if((length & 0x80) == 0) {
    //no more length bytes
  } else if((length & 0xC0) == 0x80) {
    length &^=0xC0
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
  } else if ((length & 0xE0) == 0xC0) {
    length &^=0xE0
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
  } else if ((length & 0xF0) == 0xE0) {
    length &^=0xF0
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
  } else if ((length & 0xF0) == 0xE0) {
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length = uint32(b)
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
    b, err = dev.readByte()
    if(err != nil) { dev.Close(); return "",err }
    length <<= 8
    length |= uint32(b)
  } else {
    dev.Close()
    return "",errors.New("Invalid size to read")
  }

  if(length == 0) {
    if(dev.Debug) { println("<", ":EMPTY:") }
    return "", nil
  }

  left := length
  var ret string=""

  for left > 0 {
    err := dev.conn.SetDeadline(time.Now().Add(dev.timeout))
    if(err != nil) { dev.Close(); return "",err }
    buff := make([]byte, left)
    read, err := dev.conn.Read(buff)
    if(err != nil) { dev.Close(); return "",err }
    dev.Bytes_in += uint64(read)
    if(read == 0) { dev.Close(); return "",errors.New("short read") }
    if(uint32(read) > left) { dev.Close(); return "",errors.New(fmt.Sprintf("Too much read: %d, expected: %d", read, left)) }
    select {
      case cmd := <-dev.stop_ch:
        if(cmd == "stop") {
          dev.Close();
          return "",errors.New("exit signalled")
        }
      default:
        //do nothing
    }
    ret += string(buff[:read])
    left -= uint32(read)
  }
  if(dev.Debug) { println("<", ret) }
  return ret, nil
}

func (dev *MkDev) ReadSentence(mandatory...string) (*MkSentence, error) {
  ret := new(MkSentence)
  ret.Attrs = make(map[string]string)

  var err error
  ret.Answer,err = dev.readLine()
  if(err != nil) { return nil, err }
  if(len(ret.Answer) == 0) { dev.Close(); return nil, errors.New("Empty anwser") }
  if(ret.Answer[:1] != "!") { dev.Close(); return nil, errors.New("no ! in anser: "+ret.Answer) }

  var line string
  line,err=dev.readLine()
  for err == nil && line != "" {
    if(len(line) < 3) { dev.Close(); return nil, errors.New("too short attr string") }
    eq_idx := strings.Index(line[1:], "=")
    if(eq_idx == -1) { dev.Close(); return nil, errors.New("no = in answer") }
    key := line[:eq_idx+1]
    value := line[eq_idx+2:]
    ret.Attrs[key]=value

    line,err=dev.readLine()
  }
  if(err != nil) { return nil, err }

  if(ret.Answer == "!trap") {
    dev.Close();
    trap_message, ok := ret.Attrs["=message"]
    if(ok) {
      return nil,errors.New(trap_message)
    } else {
      return nil,errors.New("!trap without message received")
    }
  }

  if(ret.Answer == "!re") {
    for _,key := range mandatory {
      _,exists := ret.Attrs[key]
      if(!exists) {
        dev.Close();
        return nil,errors.New("mandatory param missing in reply: "+key)
      }
    }
  }
  return ret, nil
}

func (dev *MkDev) SendReadDone(commands...string) (*MkSentence, error) {
  err := dev.Send(commands...)
  if(err != nil) { return nil, err }
  resp, err := dev.ReadSentence()
  if(err != nil) { return nil, err }

  if(resp.Answer == "!done") {
    return resp,nil
  } else if(resp.Answer == "!re") {
    done_resp, err := dev.ReadSentence()
    if(err != nil) { return nil, err }
    if(done_resp.Answer != "!done") {
      dev.Close();
      return nil, errors.New("Not !done answer: "+done_resp.Answer)
    }
    return resp,nil
  }
  return nil, errors.New("Wrong answer: "+resp.Answer)
}

func (dev *MkDev) Connect(user string, pass string) (error) {
  if(dev.Connected) { return errors.New("already connected") }
  var err error
  dev.conn, err = net.DialTimeout("tcp", dev.ip+":"+dev.port, dev.timeout)
  if(err != nil) { return err }
  dev.Connected=true

  resp, err := dev.SendReadDone("/login", "=name="+user, "=password="+pass)
  if(err != nil) { return err }

  challange, ok := resp.Attrs["=ret"]
  if(ok) {
    ret_bytes,err := hex.DecodeString(challange)
    if(err != nil) { dev.Close(); return err }
    pre_md5 := append([]byte{0}, []byte(pass)...)
    pre_md5 = append(pre_md5, ret_bytes...)
    md5 := md5.Sum(pre_md5)
    md5_str := hex.EncodeToString(md5[:])

    resp, err = dev.SendReadDone("/login", "=name="+user, "=response=00"+md5_str)
    if(err != nil) { return err }
  }
  return nil
}

func (snt *MkSentence) Dump() {
  println(snt.Answer)
  for key,value := range snt.Attrs {
    println("\t",key,":",value)
  }
  print("\n")
}
