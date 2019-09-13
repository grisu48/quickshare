# quickshare

[![Build Status](https://travis-ci.org/grisu48/quickshare.svg?branch=master)](https://travis-ci.org/grisu48/quickshare)

quickshare is the prototype of a simple file share utility.

It is a small webserver, where users can add files with a single call. Subsequent files are added to the running webserver, thus making it fast and simple to quickly share even larger files on the local network

## Build and run

Requirements: `go > 1.9`

    $ make
    $ sudo make install

## Usage

To share the file `README.md`

    $ quickshare README.md

Adding more files by simply running `quickshare` again

    $ quickshare LICENSE

`quickshare` runs by default on port `8249`. The local address is displayed when running the server program

    $ quickshare README.md 
    Serving: README.md (/home/phoenix/Projects/quickshare/README.md)
    2019/09/13 16:37:40 Started http://localhost:8249

# Notice

This is an early prototype. Please handle with care!
