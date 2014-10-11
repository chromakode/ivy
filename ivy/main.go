package main

import (
	"flag"
	"github.com/chromakode/ivy"
	"log"
	"net/http"
	"os"
)

func main() {
	var (
		addr   = flag.String("addr", ":8080", "http service address")
		logDir = flag.String("logs", ".", "persistence directory")
	)
	flag.Parse()

	fi, err := os.Stat(*logDir)
	if err != nil {
		log.Fatal(err)
	}
	if !fi.IsDir() {
		log.Fatalf("logdir %#v must be a directory", *logDir)
	}

	ivy := ivy.NewIvy(ivy.IvyConfig{
		LogDir: *logDir,
		LargeChunkSize: 64 * 1024,
	})
	ivy.Start()
	if err := http.ListenAndServe(*addr, ivy); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
