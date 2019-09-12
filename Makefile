default:	all
all:	quickshare

quickshare: quickshare.go
	go build quickshare.go
