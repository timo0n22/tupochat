package main

import (
	"fmt"
	"net"
)

func main() {
	conn, _ := net.Dial("tcp", "localhost:9999")
	conn.Write([]byte("let's talk?"))
	buf := make([]byte, 1024)
	conn.Read(buf)
	fmt.Println(string(buf))
}

