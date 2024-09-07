package main

import (
	"bufio"
	"fmt"
	"github.com/TwiN/go-color"
	"github.com/go-ping/ping"
	"github.com/urfave/cli/v2"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

var (
	timeout            = 300 * time.Millisecond
	start              time.Time
	startPort, endPort = 30, 1024
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "list",
				Value:   "",
				Aliases: []string{"l"},
				Usage:   "Enter a list from a text file",
			},

			&cli.StringFlag{
				Name:    "ip",
				Value:   "",
				Aliases: []string{"i"},
				Usage:   "Enter a single ip",
			},
			&cli.BoolFlag{
				Name:    "pipe",
				Aliases: []string{"p"},
				Usage:   "Enter just from a pipeline",
			},
		},
		Action: func(cCtx *cli.Context) error {
			start = time.Now()
			if cCtx.String("list") != "" {
				withList(cCtx.String("list"))
			} else if cCtx.String("ip") != "" {
				singleIP(cCtx.String("ip"))
			} else if cCtx.Bool("pipe") {
				withPipe()
			}

			elapsed := time.Since(start)
			fmt.Printf(color.Colorize(color.Red, "[*] Finished job in %s\n"), elapsed)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
func removeBOM(s string) string {
	if len(s) >= 3 && s[:3] == "\ufeff" {
		return s[3:]
	}
	return s
}
func withPipe() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		ip := scanner.Text()
		ip = removeBOM(ip)
		if _, _, err := net.ParseCIDR(ip); err == nil {
			// Process a CIDR range
			for _, ipAddr := range getIPsFromCIDR(ip) {
				if CheckHost(ipAddr) {
					fmt.Printf("%s\n", ip)
				}
			}
		} else {
			// Process a single IP
			if CheckHost(ip) {
				fmt.Printf("%s\n", ip)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func withList(inputFile string) {
	IPs := readIPS(inputFile)
	for _, ip := range IPs {

		if CheckHost(ip) {

			fmt.Printf("%s\n", ip)
		}
	}

}
func singleIP(inputIp string) {

	if CheckHost(inputIp) {

		fmt.Printf("%s\n", inputIp)
	}

}
func CheckHost(ip string) bool {
	// Check ICMP availability
	if PingIP(ip) {
		return true
	}
	// If ICMP fails, check TCP ports to confirm if the host is actually reachable
	return CheckTCPPorts(ip, startPort, endPort)
}

func PingIP(ip string) bool {

	pinger, err := ping.NewPinger(ip)
	if err != nil {
		log.Printf("Failed to create pinger: %v", err)
		return false
	}
	pinger.Count = 1
	pinger.Timeout = 1 * time.Second
	pinger.Run()
	stats := pinger.Statistics()
	return stats.PacketsRecv > 0
}

func CheckTCPPorts(ip string, startPort, endPort int) bool {
	var wg sync.WaitGroup
	results := make(chan bool, endPort-startPort+1)

	for port := startPort; port <= endPort; port++ {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			address := fmt.Sprintf("%s:%d", ip, port)
			conn, err := net.DialTimeout("tcp", address, timeout)
			if err != nil {
				results <- false
				return
			}
			conn.Close()
			results <- true
		}(port)
	}

	go func() {
		wg.Wait()
		close(results) // Close the channel after all goroutines have finished
	}()

	// Collect results
	for result := range results {
		if result {
			return true
		}
	}

	return false
}

func readIPS(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var ips []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = removeBOM(line)
		// Handle both CIDR ranges and individual IPs
		if _, _, err := net.ParseCIDR(line); err == nil {
			ips = append(ips, getIPsFromCIDR(line)...)
		} else {
			ips = append(ips, line)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return ips
}
func getIPsFromCIDR(cidr string) []string {
	var ips []string
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatalf("Invalid CIDR: %v", err)
	}
	ip := ipnet.IP
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
		ips = append(ips, ip.String())
	}
	return ips
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}
