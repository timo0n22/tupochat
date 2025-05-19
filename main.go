package main

import (
	"bufio"
	"fmt"
	"net"
)

func HandleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		n, _ := reader.ReadString('\n')
		if n == "/exit\n" {
			fmt.Println("client exit")
			break
		}
		fmt.Println("client msg:", n)
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	for {
		conn, _ := ln.Accept()
		go HandleConnection(conn)
	}
	fmt.Println("server exit")
}
