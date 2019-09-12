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
func getShare(filename string) (Share) {
    for _, share := range Shares {
        if share.Filename == filename { return share }
    }
    share := Share{Name: "", Filename: ""}
    return share
}

/** Send the given share to the client */
func sendFile(w http.ResponseWriter, share Share) (error) {
    file, err := os.Open(share.Filename)
    if err != nil {
        file.Close()
        return err
    }
    
    buffer := make([]byte, 8192)
    bytes := 0
    for {
        count, err := file.Read(buffer)
        if err == io.EOF { break }
        if err != nil {
            file.Close()
            return err
        }
        w.Write(buffer[:count])
        bytes += count
    }
    
    file.Close()
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
                fmt.Fprintf(w, "<li><a href=\"%s\">%s</a></li>\n", share.Name, share.Filename)
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



func main() {
	port := 8249        // Port to be used
	
	for _, arg := range os.Args[1:] {
	    share := Share{Name: arg, Filename: arg}
	    Shares = append(Shares, share)
	    fmt.Printf("Serving: %s\n", arg)
	}
	
	http.HandleFunc("/", httpHandler);
	log.Printf("Started http://localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
