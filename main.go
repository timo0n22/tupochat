package main

import (
	"fmt"
	"net"
)

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	conn, _ := ln.Accept()
	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	fmt.Println("Получено:", string(buf[:n]))
	conn.Write([]byte("no"))
}
