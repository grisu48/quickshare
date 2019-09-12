/*
 * Quickshare - A small utility for fast file sharing
 */
 
package main

import (
    "os"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"net"
	"bufio"
	"os/signal"
	"syscall"
	"time"
)

type Share struct {
    Name string
    Filename string
    Timeout int64
}


type Host struct {
	Name string
	Shares int
}


func getFilename(filename string) (string) {
    i := strings.LastIndex(filename, "/")
    if i < 0 { return filename }
    return filename[i+1:]
}

func abspath(filename string) (string) {
	if filename == "" { return "" }
	if filename[0] == '/' { return filename }
	cwd, err := os.Getwd()
	if err != nil { panic(err) }
	if cwd[len(cwd)-1] != '/' { cwd += "/" }
	return cwd + filename
}

/** Apply a PATH. PATH can be either a pathname or name:pathname */
func (s *Share) apply(pathname string) {
	i := strings.Index(pathname, ":")
	if i < 0 {
		s.Name = getFilename(pathname)
		s.Filename = abspath(pathname)
	} else {
		s.Name = pathname[:i]
		s.Filename = abspath(pathname[i+1:])
	}
}

func (s *Share) ShareName() (string) {
	if s.Name != "" {
		return fmt.Sprintf("%s:%s", s.Name, s.Filename)
	} else {
		return s.Filename
	}
}

var Shares []Share
var Verbose int


/** Get the filename for a request URI */
func getHttpFilename(uri string) (string) {
    i := strings.Index(uri, "/")
    if i < 0 { return "" }
    filename := uri[i+1:]
    return filename
}


/** Find a share */
func getShare(name string) (Share) {
    for _, share := range Shares {
        if share.Name == name { return share }
    }
    share := Share{Name: "", Filename: ""}
    return share
}

func ShareExists(name string) (bool) {
    for _, share := range Shares {
    	if share.Name == name { return true }
    }
    return false
}

func RemoveShare(name string) (bool) {
    for i, share := range Shares {
    	if share.Name == name {
    		// This is slow. But I want to keep the order for now
    		Shares = append(Shares[:i], Shares[i+1:]...)
    		return true
    	}
    }
    return false
}

/** Send the given share to the client */
func sendFile(w http.ResponseWriter, share Share) (error) {
	stats, err := os.Stat(share.Filename);	
	if err != nil { return err }
	
    file, err := os.Open(share.Filename)
    defer file.Close()
    if err != nil { return err }
    
    // All good so far. Write headers
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", share.Name))
    w.Header().Set("Content-Length", fmt.Sprintf("%d", stats.Size()))
    
    buffer := make([]byte, 8192)
    bytes := 0
    for {
        count, err := file.Read(buffer)
        if err == io.EOF { break }
        if err != nil {
            return err
        }
        w.Write(buffer[:count])
        bytes += count
    }
    
    return nil
}



/** General HTTP handler */
func httpHandler(w http.ResponseWriter, r *http.Request) {
	if Verbose > 0 { log.Printf("%s GET %s", r.RemoteAddr, r.RequestURI) }
    filename := getHttpFilename(r.RequestURI)
    
    if (filename == "" || filename == "index.html" || filename == "index.html") {
        fmt.Fprintf(w, "<h1>Quickshare File Server</h1>\n")
        
        if len(Shares) == 0 {
            fmt.Fprintf(w, "<p>No shares on this server</p>\n")
        } else {
            fmt.Fprintf(w, "<p>%d share(s) on this server:\n<ul>\n", len(Shares))
            for _, share := range Shares {
                fmt.Fprintf(w, "<li><a href=\"%s\">%s</a></li>\n", share.Name, share.Name)
            }
            
            fmt.Fprintf(w, "</ul></p>\n")
        }
    } else {
        share := getShare(filename)
        if share.Name == "" {
            // Not found
            w.WriteHeader(http.StatusNotFound)
            w.Write([]byte("Object not found"))
            return
        } else {
            if Verbose == 0 { log.Printf("  %s GET %s\n", r.RemoteAddr, filename) }
            
            err := sendFile(w, share)
            if err != nil {
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte("Server error"))
                fmt.Fprintf(os.Stderr, "Error sending file: %s\n", err)
            }
        }
    }
}



func serverHandle(c net.Conn) {
	reader := bufio.NewReader(c)
	
	for {
		bline, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF { break }
			log.Printf("Error reading from unix socket: %s\n", err)
			break
		}
		line := strings.TrimSpace(string(bline))
		if Verbose > 1 { fmt.Printf("READ %s\n", line) }
		
		if line == "ping" {
			c.Write([]byte("pong\n"))
		} else if strings.HasPrefix(line, "add ") {
			line := line[4:]
			share := Share{}
			share.apply(line)
			
			if ShareExists(share.Name) {
				c.Write([]byte("ERR Share exists already\n"))
			} else {			
				// TODO: Check if file exists
				
				Shares = append(Shares, share)
				c.Write([]byte(fmt.Sprintf("OK Share \"%s\"@'%s'\n", share.Name, share.Filename)))
				log.Printf("Added share \"%s\"@'%s'\n", share.Name, share.Filename)
			}
		} else if strings.HasPrefix(line, "rm ") {
			name := line[3:]
			
			if RemoveShare(name) {
				c.Write([]byte("OK\n"))
			} else {
				c.Write([]byte("ERR Share not found\n"))
			}
		} else if line == "ls" || line == "list" {
			for _, share := range Shares {
				c.Write([]byte(fmt.Sprintf("%s %s\n", share.Name, share.Filename)))
			}
			c.Write([]byte("OK\n"))
		} else if line == "stop" {
			c.Write([]byte("OK\n"))
			c.Close()
			// Quit server
			log.Printf("Shutting down server")
			Exit(0)
		}
	}
	c.Close()
}

func serverHandler(socket net.Listener) {
	for {
		fd, err := socket.Accept()
		if err != nil {
			log.Printf("Error accepting unix client: %s\n", err)
			continue
		} else {
			go serverHandle(fd)
		}
	}
}

func Exit(code int) {
	os.RemoveAll("/var/tmp/quickshare")
	os.Exit(code)
}


func exists(pathname string) (bool) {
	if _, err := os.Stat(pathname); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func broadcastUdp(con *net.UDPConn) {
	for {
		if Verbose > 1 { fmt.Printf("Sending broadcast ... \n") }
		_, err := con.Write([]byte("DISCOVER"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending broadcast: %s\n", err)
			return
		}
		time.Sleep(1*time.Second)
	}
}

func runDiscover(port int) {
	fmt.Println("Discovering servers in network ... ")
	
	// Register signal handlers
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		os.Exit(0)
	}()
	
	//broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", port))
	broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving UDP broadcast address: %s\n", err)
	}
	con, err := net.DialUDP("udp", nil, broadcastAddr)
	defer con.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error binding to UDP broadcast address: %s\n", err)
		os.Exit(1)
	}
	go broadcastUdp(con)
	
	buffer := make([]byte, 2048)
	if Verbose > 0 { fmt.Println("Waiting for incoming messages ... ") }
	for {
		rlen, remote, err := con.ReadFrom(buffer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error receiving udp packet: %s\n", err)
		} else {
			data := strings.TrimSpace(string(buffer[:rlen]))
			fmt.Printf("  - %s (http://%s)\n", data, remote)
		}
	}
}

func udpServer(port int) {
	udp, err := net.ListenUDP("udp", &net.UDPAddr{ Port: port })
	defer udp.Close()
	if err != nil {
		log.Printf("Error creating upd server: %s\n", err)
		return		// This error is non-critical, so just exit the thread
	}
	
	hostname, err := os.Hostname()
	if err != nil { hostname = "unknown" }

	if Verbose > 0 { log.Printf("UDP server started on %s\n", udp.LocalAddr().String()) }
	for {
		message := make([]byte, 2048)
		rlen, remote, err := udp.ReadFromUDP(message[:])
		if err != nil {
			log.Printf("UDP receive error: %s\n", err)
			break
		}
		data := strings.TrimSpace(string(message[:rlen]))
		
		if Verbose > 1 { log.Printf("UDP RECEIVE %s : %s\n", remote, data) }
		
		if data == "DISCOVER" {
			// Reply
			udp.WriteTo([]byte(hostname), remote)
		}
	}
}

func main() {
	port := 8249        // Port to be used
	u_socket := "/var/tmp/quickshare"
	Verbose = 0
	files := make([]string, 0)
	
	// Handle special arguments
	for _, arg := range(os.Args[1:]) {
		if arg == "" { continue }
		
		if arg[0] == '-' {
			if arg == "-h" || arg == "--help" {
				fmt.Println("quickshare - Quick file share server")
				fmt.Println("  2019, Felix Niederwanger\n")
				fmt.Printf("Usage: %s [OPTIONS] [FILES]\n", os.Args[0])
				fmt.Println("OPTIONS")
				fmt.Println("  -h, --help               Print this help message")
				fmt.Println("  -v, --verbose            Verbose (-vv increases verbosity)")
				fmt.Println("  --ls                     List all current shares")
				fmt.Println("  --stop                   Stop the server")
				fmt.Println("")
				fmt.Println("  --discover               Search the network for shares")
				os.Exit(0)
			} else if arg == "-v" || arg == "--verbose" {
				Verbose = 1
			} else if arg == "-vv" || arg == "--verboseverbose" {
				Verbose = 2
			} else if arg == "--discover" {
				runDiscover(port)
				os.Exit(0)
			}
		} else {
			files = append(files, arg)
		}
	}
	
	if exists(u_socket) {
		// Enter client mode, as the server already is running
		if Verbose > 0 { fmt.Printf("Connecting to server '%s' ... \n", u_socket) }
		sock, err := net.Dial("unix", u_socket)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to server socket: %s\n", err)
			os.Exit(1)
		}
		if Verbose > 0 { fmt.Printf("Connected to unix socket \n") }
		reader := bufio.NewReader(sock)
		
		// Client commands
		for _, arg := range(os.Args[1:]) {
			arg = strings.TrimSpace(arg)
			if arg == "" { continue }
			if arg[0] == '-' {
				if arg == "--ls" {
				sock.Write([]byte(fmt.Sprintf("ls\n", arg)))
				for {
					bline, _, err := reader.ReadLine()
					if err != nil {
						if err == io.EOF { break }
						log.Printf("Error reading from unix socket: %s\n", err)
						continue
					}
					line := strings.TrimSpace(string(bline))
					if line == "OK" { break }
					fmt.Printf("  - %s\n", line)
				}
					
				} else {
					fmt.Printf("Error: Unknown argument '%s'\n", arg)
					os.Exit(1)
				}
			}
		}
		
		for _, filename := range(files) {
			share := Share{}
			share.apply(filename)
			
			// Add share
			sock.Write([]byte(fmt.Sprintf("add %s\n", share.ShareName())))
				
			bline, _, err := reader.ReadLine()
			if err != nil {
				if err == io.EOF { break }
				log.Printf("Error reading from unix socket: %s\n", err)
				continue
			}
			line := strings.TrimSpace(string(bline))
			
			if strings.HasPrefix(line, "OK") {
				if Verbose >= 0 { fmt.Printf("Serving: %s (%s)\n", share.Name, share.Filename) }
			} else if strings.HasPrefix(line, "ERR ") {
				fmt.Printf("Error adding share '%s': %s\n", filename, line[4:])
			} else {
				fmt.Printf("Unknwon response when adding share '%s': %s\n", filename, line)
			}
		}
		
		sock.Close()
	} else {
		// Server mode
		sock, err := net.Listen("unix", u_socket)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating server socket: %s\n", err)
			os.Exit(1)
		}
		go serverHandler(sock)
		if Verbose > 0 { fmt.Println("Unix socket: /var/tmp/quickshared") }
		
		
		// Register signal handlers
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigs
			fmt.Println(sig)
			Exit(2)
		}()
		
		// Add files
		for _, filename := range files {
			share := Share{}
			share.apply(filename)
			Shares = append(Shares, share)
			if Verbose >= 0 { fmt.Printf("Serving: %s (%s)\n", share.Name, share.Filename) }
		}
		
		// UDP server
		go udpServer(port)
		
		http.HandleFunc("/", httpHandler);
		log.Printf("Started http://localhost:%d\n", port)
		http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}
}
