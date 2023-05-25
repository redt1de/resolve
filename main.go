package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	useStdin bool
	maxCon   int
	ipv6     bool
	posArg   string
	s        string
	servers  []string
	targets  []string
	wg       sync.WaitGroup
)

func main() {
	flag.StringVar(&s, "s", "", "comma seperated list of custom nameservers")
	flag.BoolVar(&ipv6, "6", false, "include IPv6 addresses")
	flag.IntVar(&maxCon, "c", 10, "max concurrency")
	flag.Usage = func() {
		flagSet := flag.CommandLine

		fmt.Println("Usage:\nresolve [options] <file|ip|hostname|stdin>")
		order := []string{"s", "6", "c"}
		for _, name := range order {
			flag := flagSet.Lookup(name)
			fmt.Printf("\t-%s\t%s\n", flag.Name, flag.Usage)
		}
	}

	flag.Parse()

	if s != "" {
		s = strings.ReplaceAll(s, " ", "")
		servers = strings.Split(s, ",")
	}

	sem := make(chan int, maxCon)

	if flag.NArg() == 0 {
		useStdin = true
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			sem <- 1
			wg.Add(1)
			go func(line string) {
				doResolve(line)
				<-sem
				wg.Done()
			}(scanner.Text())
		}

		if scanner.Err() != nil {
			log.Fatal(scanner.Err())
		}

	} else {
		posArg = flag.Args()[0]
		var err error
		if fileExists(posArg) {
			targets, err = readLines(posArg)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			targets = append(targets, posArg)
		}

		for _, line := range targets {
			sem <- 1
			wg.Add(1)
			go func(l string) {
				doResolve(l)
				<-sem
				wg.Done()
			}(line)
		}

	}
	wg.Wait()

}

func doResolve(targ string) {
	if isIPAddress(targ) {
		lookupReverse(targ)
	} else {
		lookup(targ)
	}
}

func lookup(targ string) {
	var lst []net.IP
	useNet := "ip4"
	if len(servers) > 0 {
		for _, curserv := range servers {
			r := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{
						Timeout: time.Millisecond * time.Duration(10000),
					}
					return d.DialContext(ctx, network, curserv+":53")
				},
			}
			if ipv6 {
				useNet = "ip"
			}
			lst, _ = r.LookupIP(context.Background(), useNet, targ)
			if len(lst) > 0 {
				break
			}
		}

	} else {
		lst, _ = net.LookupIP(targ)
	}

	if len(lst) == 0 {
		fmt.Println(targ + ":")
		return
	}
	for _, i := range lst {
		if !ipv6 && strings.Contains(i.String(), ":") {
			continue
		}
		fmt.Println(targ + ":" + i.String())
	}

}

func lookupReverse(targ string) {
	var lst []string
	if len(servers) > 0 {
		for _, curserv := range servers {
			r := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{
						Timeout: time.Millisecond * time.Duration(10000),
					}
					return d.DialContext(ctx, network, curserv+":53")
				},
			}
			lst, _ = r.LookupAddr(context.Background(), targ)
			if len(lst) > 0 {
				break
			}
		}

	} else {
		lst, _ = net.LookupAddr(targ)
	}

	if len(lst) == 0 {
		fmt.Println(":" + targ)
		return
	}
	for _, i := range lst {

		fmt.Println(strings.TrimRight(i, ".") + ":" + targ)
	}
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// WriteLines writes the lines to the given file.
func WriteLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func isIPAddress(ip string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}
	return true
}
