package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
)


func main() {
	const PORT = "8080"
	server, err := net.Listen("tcp", ":" + PORT);

	if err != nil {
		panic(err);
	}

	defer server.Close();
	color.Green("[+] Server has been started on port %s", PORT );

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go waitForSignals(server, sigChan);

	for {
		conn, err := server.Accept();

		if err != nil {
			color.Red("[x] Unable to accept connection. ")
			color.Red("[x] Error: " + err.Error())
			break;
		}

		go connectionHandler(conn);
	}

}


func connectionHandler(conn net.Conn) {
	defer conn.Close();

	buffer := make([]byte, 1024 * 5); // 5 KB buffer for incoming messages

	for {
		n, err := conn.Read(buffer);
		if err != nil {
			color.Blue("[-] Client disconnected");
			continue;
		}

		data := buffer[:n]
		fmt.Printf("[+] Received data: %s\n", data);

		conn.Write([]byte("Echoed back: " + string(data)))

	}

}

func waitForSignals( server net.Listener, sigChan chan os.Signal) {
	<- sigChan;
	color.Red("[!] Closing the server");
	server.Close();
}