// +build linux

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/elastic/gosigar/sys/linux"
)

var (
	fs    = flag.NewFlagSet("ss", flag.ExitOnError)
	debug = fs.Bool("d", false, "enable debug output to stderr")
	ipv6  = fs.Bool("6", false, "display only IP version 6 sockets")
	v1    = fs.Bool("v1", false, "send inet_diag_msg v1 instead of v2")
	diag  = fs.String("diag", "", "dump raw information about TCP sockets to FILE")
)

func enableLogger() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})
}

func main() {
	fs.Parse(os.Args[1:])

	if *debug {
		enableLogger()
	}

	if err := sockets(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func sockets() error {
	// Set address family based on flags. The requested address family only
	// works with inet_diag_req_v2. v1 returns all tcp sockets.
	af := linux.AF_INET
	if *ipv6 {
		af = linux.AF_INET6
	}

	// For debug purposes allow for sending either inet_diag_req and inet_diag_req_v2.
	var req syscall.NetlinkMessage
	if *v1 {
		req = linux.NewInetDiagReq()
	} else {
		req = linux.NewInetDiagReqV2(af)
	}

	// Write netlink response to a file for further analysis or for writing
	// tests cases.
	var diagWriter io.Writer
	if *diag != "" {
		f, err := os.OpenFile(*diag, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		diagWriter = f
	}

	log.Debugln("sending netlink request")
	msgs, err := linux.NetlinkInetDiagWithBuf(req, nil, diagWriter)
	if err != nil {
		return err
	}
	log.Debugf("received %d inet_diag_msg responses", len(msgs))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join([]string{
		"State",
		"Recv-Q",
		"Send-Q",
		"Local Address:Port",
		"Remote Address:Port",
		"UID",
		"Inode",
		"PID/Program",
	}, "\t"))
	defer w.Flush()

	for _, diag := range msgs {
		// XXX: A real implementation of ss would find the process holding
		// inode of the socket. It would read /proc/<pid>/fd and find all sockets.
		pidProgram := "not implemented"

		fmt.Fprintf(w, "%v\t%v\t%v\t%v:%v\t%v:%v\t%v\t%v\t%v\n",
			linux.TCPState(diag.State), diag.RQueue, diag.WQueue,
			diag.SrcIP().String(), diag.SrcPort(),
			diag.DstIP().String(), diag.DstPort(),
			diag.UID, diag.Inode, pidProgram)
	}

	return nil
}
