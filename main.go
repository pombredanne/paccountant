package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var log_filename = flag.String("output", "paccountant.log", "Log filename")

type Process struct {
	Cmdline, Pwd, Exe string
	Status            int
	Uid               int64
	When              time.Time

	Memory struct {
		Maxrss uint32
	}

	Io struct {
		Nreads, Nwrites uint64
		Byter, Bytew    uint64
		Blockr, Blockw  uint64 // To block devices
	}
}

func makeDict(input string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(input, "\n") {
		leftright := strings.SplitN(line, ":", 2)
		if len(leftright) < 2 {
			continue
		}
		result[leftright[0]] = strings.TrimSpace(leftright[1])
	}
	return result
}

func NewProcess(when time.Time, status int, cmdline, pwd, exe, io,
	stat string) Process {

	iodict := makeDict(io)
	statdict := makeDict(stat)

	process := Process{
		Cmdline: cmdline,
		Status:  status,
		Pwd:     pwd,
		Exe:     exe,
		When:    when,
	}

	fmt.Sscan(statdict["Uid"], &process.Uid)

	fmt.Sscan(statdict["VmHWM"], &process.Memory.Maxrss)

	fmt.Sscan(iodict["rchar"], &process.Io.Byter)
	fmt.Sscan(iodict["wchar"], &process.Io.Bytew)
	fmt.Sscan(iodict["read_bytes"], &process.Io.Blockr)
	fmt.Sscan(iodict["write_bytes"], &process.Io.Blockw)
	fmt.Sscan(iodict["syscr"], &process.Io.Nreads)
	fmt.Sscan(iodict["syscw"], &process.Io.Nwrites)

	return process
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func serveOne(conn net.Conn, data chan<- Process) {

	when := time.Now()

	// Ensure that ``conn`` is closed on matter how we exit serveOne()
	exit := func() {
		// log.Println("Done")
		err := conn.Close()
		check(err)
	}

	defer exit()

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

	cmdline_content, err := ioutil.ReadFile(proc + "cmdline")
	check(err)

	pwd, err := os.Readlink(proc + "cwd")
	check(err)
	exe, err := os.Readlink(proc + "exe")
	check(err)

	go func() {
		data <- NewProcess(when, status, string(cmdline_content), pwd, exe,
			string(io_content), string(status_content))
	}()
}

func writelog(data <-chan Process, done <-chan struct{}, hup <-chan os.Signal) {

	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	fd, err := os.OpenFile(*log_filename, flags, 0666)
	check(err)

	defer log.Println("Done")
	// Inside func(){} To ensure ``fd`` is bound correctly after SIGHUP
	defer func() { fd.Close() }()

	buf := &bytes.Buffer{}

	for {
		select {
		case <-hup:
			log.Println("SIGHUP")
			err = fd.Close()
			check(err)
			fd, err = os.OpenFile(*log_filename, flags, 0666)
			check(err)
			continue

		case <-done:
			return

		case datum := <-data:
			bytes, err := json.Marshal(&datum)
			check(err)

			n, err := buf.Write(bytes)
			check(err)

			buf.WriteByte('\n')

			if n != len(bytes) {
				err = fmt.Errorf(
					"Unexpected number of bytes to buffer.. %v != %v",
					n, len(bytes))
				check(err)
			}

			_, err = io.Copy(fd, buf)
			check(err)
		}
	}
}

func main() {

	// TODO(pwaller): if GOMAXPROCS isn't specified in the environment, switch
	// it to at least 2.

	listener, err := net.Listen("tcp4", "localhost:7117")
	check(err)
	defer listener.Close()

	if os.Getuid() != 0 {
		log.Println("Not running as root. Some processes may be invisible.")
	}

	logChan := make(chan Process)
	done := make(chan struct{})

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()

		hup := make(chan os.Signal)
		signal.Notify(hup, syscall.SIGHUP)
		writelog(logChan, done, hup)
	}()

	nConnections := int32(0)
	const MAX_OUTSTANDING = 5

	go func() {
		for {
			conn, err := listener.Accept()
			check(err)

			if atomic.LoadInt32(&nConnections) > MAX_OUTSTANDING {
				// Last ditch saftey to prevent holding too many inbound
				// connections simultaneously.
				conn.Close()
				log.Print("Last chance safety was hit nmax=%v", MAX_OUTSTANDING)
				continue
			}

			atomic.AddInt32(&nConnections, 1)
			go func() {
				defer atomic.AddInt32(&nConnections, -1)

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
	signal.Notify(s, os.Interrupt)
	<-s

	log.Println("Interrupted")

	close(done)
}
