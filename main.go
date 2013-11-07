package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func serveOne(conn net.Conn, data chan<- []byte) {
	// Ensure that ``conn`` is closed on matter how we exit serveOne()
	defer func() {
		err := conn.Close()
		check(err)
	}()

	b := make([]byte, 32)
	n, err := conn.Read(b)
	check(err)

	s := string(b[:n-1])
	log.Printf("Connection from %q", s)

	var pid uint64
	var status int

	_, err = fmt.Sscanln(s, &pid, &status)
	check(err)

	proc := fmt.Sprintf("/proc/%v/", pid)

	io_content, err := ioutil.ReadFile(proc + "io")
	check(err)

	status_content, err := ioutil.ReadFile(proc + "status")
	check(err)

	cmdline, err := ioutil.ReadFile(proc + "cmdline")
	check(err)

	go func() {
		data <- append(status_content, append(io_content, cmdline...)...)
	}()
}

func writelog(data <-chan []byte, done <-chan struct{}) {
	defer println("Written log")

	fd, err := os.Create("paccountant.log")
	check(err)
	defer fd.Close()

	buf := &bytes.Buffer{}
	var datum []byte

	for {
		select {
		case datum = <-data:
		case <-done:
			log.Println("Recv <-done")
			return
		}

		n, err := buf.Write(datum)
		check(err)
		if n != len(datum) {
			check(fmt.Errorf("Unexpected number of bytes to buffer.. %v != %v",
				n, len(datum)))
		}

		_, err = io.Copy(fd, buf)
		check(err)

		fd.Write([]byte{0})
	}
}

func main() {
	listener, err := net.Listen("tcp4", "localhost:7117")
	check(err)
	defer listener.Close()

	logChan := make(chan []byte)
	done := make(chan struct{})

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		writelog(logChan, done)
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			check(err)

			go func() {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("serveOne failed %v", err)
					}
				}()
				serveOne(conn, logChan)
			}()
		}
	}()

	s := make(chan os.Signal)
	signal.Notify(s)
	<-s

	close(done)
}
