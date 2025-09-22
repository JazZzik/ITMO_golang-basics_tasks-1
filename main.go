package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	statsURL = "http://srv.msk01.gigacorp.local/_stats"

	// default settings
	pollingInterval = 5 * time.Second
	httpTimeout     = 30 * time.Second

	// task settings
	loadAverageThreshold      = 30
	memoryUsageThreshold      = 80
	freeDiscSpaceThreshold    = 90
	networkBandwidthThreshold = 90
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// init http client
	client := &http.Client{Timeout: httpTimeout}

	// init ticker
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := pollOnce(client)
			if err != nil {
				fmt.Println("Unable to fetch server statistic.")
			}
		}
	}
}

func pollOnce(client *http.Client) error {
	req, _ := http.NewRequest("GET", statsURL, nil)

	// req
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// check resp satus code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// get resp body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// get resp parts and check for len
	line := strings.TrimSpace(string(bodyBytes))
	parts := splitCSV(line)
	if len(parts) != 7 {
		return fmt.Errorf("unexpected field number: %d", len(parts))
	}

	errNum := 0

	// get data
	loadAvg, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		errNum++
	}

	memTotal, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		errNum++
	}
	memUsed, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		errNum++
	}
	diskTotal, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		errNum++
	}
	diskUsed, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		errNum++
	}
	netCap, err := strconv.ParseUint(parts[5], 10, 64)
	if err != nil {
		errNum++
	}
	netUsed, err := strconv.ParseUint(parts[6], 10, 64)
	if err != nil {
		errNum++
	}

	if errNum > 3 {
		return fmt.Errorf("too may errors")
	}

	// 1) Load Average
	if loadAvg > loadAverageThreshold {
		fmt.Printf("Load Average is too high: %d\n", loadAvg)
	}

	// 2) Memory usage >80%
	if memTotal == 0 {
		return fmt.Errorf("memTotal=0")
	}
	memPct := (float64(memUsed) / float64(memTotal)) * 100.0
	if memPct > memoryUsageThreshold {
		fmt.Printf("Memory usage too high: %f%%\n", memPct)
	}

	// 3) Disk usage
	if diskTotal == 0 {
		return fmt.Errorf("diskTotal=0")
	}
	diskPct := (float64(diskUsed) / float64(diskTotal)) * 100.0
	if diskPct > freeDiscSpaceThreshold {
		freeBytes := int64(diskTotal - diskUsed)
		freeMB := freeBytes / (1024 * 1024)
		fmt.Printf("Free disk space is too low: %d Mb left\n", freeMB)
	}

	// 4) Network usage
	if netCap == 0 {
		return fmt.Errorf("netCap=0")
	}
	netPct := (float64(netUsed) / float64(netCap)) * 100.0
	if netPct > networkBandwidthThreshold {
		freeBytesPerSec := float64(netCap - netUsed)
		freeMbit := (freeBytesPerSec * 8.0) / 1_000_000.0
		fmt.Printf("Network bandwidth usage high: %f Mbit/s available\n", freeMbit)
	}

	return nil
}

func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		} else {
			out = append(out, p)
		}
	}
	return out
}
