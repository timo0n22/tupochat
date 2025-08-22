package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

type Client struct {
	conn net.Conn
	id   int
	name string
}

var clients []Client
var id = 0

func parseMessage(msg string) (string, string) {
	split := strings.Split(msg, ":")
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", split[0]
}

func distribute(from string, message string) {
	msg := fmt.Sprintf("%s: %s", from, message)
	for _, client := range clients {
			client.conn.Write([]byte(msg))
	}
}

func handleConnection(c Client) {
	defer c.conn.Close()
	reader := bufio.NewReader(c.conn)
	for {
		data, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%s disconnected\n", c.name)
			return
		}
	  tag, msg := parseMessage(data)
		if tag == "name" {
			old := c.name
			c.name = msg
			fmt.Printf("client %d, %s changed name to %s\n", c.id, old, c.name)
		}
		if msg == "/exit\n" {
			fmt.Printf("client %d, %s exit chat\n", c.id, c.name)
			return
		}
		fmt.Printf("%s: %s", c.name, msg)
		distribute(c.name, msg)
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			msg, _ := reader.ReadString('\n')
			fmt.Printf("server: %s", msg)
			distribute("server", msg)
		}
	}()
	for {
		conn, _ := ln.Accept()
		id++
		name, _ := bufio.NewReader(conn).ReadString('\n')
		name = strings.TrimSuffix(name, "\n")
		clients = append(clients, Client{conn, id, name})
		go handleConnection(Client{conn, id, name})
	}
}
