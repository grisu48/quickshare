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
)

type Share struct {
    Name string
    Filename string
    Timeout int64
}

var Shares []Share


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
	stats, err := os.Stat("/path/to/file");	
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
            log.Printf("  %s GET %s\n", r.RemoteAddr, filename)
            err := sendFile(w, share)
            if err != nil {
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte("Server error"))
                fmt.Fprintf(os.Stderr, "Error sending file: %s\n", err)
            }
        }
    }
}


func getFilename(filename string) (string) {
    i := strings.LastIndex(filename, "/")
    if i < 0 { return filename }
    return filename[i+1:]
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
		
		if line == "ping" {
			c.Write([]byte("pong\n"))
		} else if strings.HasPrefix(line, "add ") {
			line := line[4:]
			share := Share{Name: strings.TrimSpace(getFilename(line)), Filename: strings.TrimSpace(line) }
			i := strings.Index(line, " ")
			if i > 0 {
				share.Name = strings.TrimSpace(line[:i])
				share.Filename = line[i+1:]
			}
			
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

func abspath(filename string) (string) {
	if filename == "" { return "" }
	if filename[0] == '/' { return filename }
	cwd, err := os.Getwd()
	if err != nil { panic(err) }
	if cwd[len(cwd)-1] != '/' { cwd += "/" }
	return cwd + filename
}

func main() {
	port := 8249        // Port to be used
	u_socket := "/var/tmp/quickshare"
	
	if exists(u_socket) {
		// Enter client mode, as the server already is running
		sock, err := net.Dial("unix", u_socket)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to server socket: %s\n", err)
			os.Exit(1)
		}
		reader := bufio.NewReader(sock)
		
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
			} else {
				// Convert to absolute
				pathname := abspath(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error accessing %s: %s\n", arg, err)
					os.Exit(1)
				}
				// Add share
				sock.Write([]byte(fmt.Sprintf("add %s\n", pathname)))
				
				bline, _, err := reader.ReadLine()
				if err != nil {
					if err == io.EOF { break }
					log.Printf("Error reading from unix socket: %s\n", err)
					continue
				}
				line := strings.TrimSpace(string(bline))
				
				if strings.HasPrefix(line, "OK") {
					fmt.Printf("Added share: %s\n", arg)
				} else if strings.HasPrefix(line, "ERR ") {
					fmt.Printf("Error adding share '%s': %s\n", arg, line[4:])
				} else {
					fmt.Printf("Unknwon response when adding share '%s': %s\n", arg, line)
				}
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
		fmt.Println("Unix socket: /var/tmp/quickshared")
		
		
		// Register signal handlers
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigs
			fmt.Println(sig)
			Exit(2)
		}()
		
		
		for _, arg := range os.Args[1:] {
			share := Share{Name: arg, Filename: arg}
			Shares = append(Shares, share)
			fmt.Printf("Serving: %s\n", arg)
		}
		
		http.HandleFunc("/", httpHandler);
		log.Printf("Started http://localhost:%d\n", port)
		http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}
}
