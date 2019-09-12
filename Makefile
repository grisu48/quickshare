default:	all
all:	quickshare

quickshare: quickshare.go
	go build quickshare.go

install:	quickshare
	install quickshare /usr/local/bin/quickshare
