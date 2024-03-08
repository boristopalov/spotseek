package peer

import (
    "net"
    "spotseek/src/slskClient/client/serverListener"
)

type Peer struct { 
	Username string
	Listener net.Listener
	Conn *serverListener.ServerListener
	ConnType string
	Token uint32
	Host string
	Port uint32
}
