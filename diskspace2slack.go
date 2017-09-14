package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/nlopes/slack"
)

const (
	BYTE     = 1.0
	KILOBYTE = 1024 * BYTE
	MEGABYTE = 1024 * KILOBYTE
	GIGABYTE = 1024 * MEGABYTE
	TERABYTE = 1024 * GIGABYTE
)

// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
func ByteSize(bytes uint64) string {
	unit := ""
	value := float32(bytes)
	switch {
	case bytes >= TERABYTE:
		unit = "TB"
		value = value / TERABYTE
	case bytes >= GIGABYTE:
		unit = "GB"
		value = value / GIGABYTE
	case bytes >= MEGABYTE:
		unit = "MB"
		value = value / MEGABYTE
	case bytes >= KILOBYTE:
		unit = "KB"
		value = value / KILOBYTE
	case bytes >= BYTE:
		unit = "B"
	case bytes == 0:
		return "0"
	}

	stringValue := fmt.Sprintf("%.1f", value)
	stringValue = strings.TrimSuffix(stringValue, ".0")
	return fmt.Sprintf("%s%s", stringValue, unit)
}

// DiskState represents available/used/free space on drive
type DiskState struct {
	Host           string
	Name           string
	All            uint64
	Used           uint64
	Free           uint64
	FreePercentage uint64
}

// StatDisk calculates the disk usage of path/disk
func StatDisk(path string) (DiskState, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return DiskState{}, errors.New("Couldn't stat path " + path)
	}
	localDisk := DiskState{}
	localDisk.All = fs.Blocks * uint64(fs.Bsize)
	localDisk.Free = fs.Bavail * uint64(fs.Bsize)
	localDisk.FreePercentage = uint64(float32(localDisk.Free) / float32(localDisk.All) * 100)
	localDisk.Used = localDisk.All - localDisk.Free
	host, err := os.Hostname()
	if err != nil {
		fmt.Print("Unable to get hostname. Using `Unknown`.")
		host = "Unknown"
	}

	localDisk.Name = path
	localDisk.Host = host
	return localDisk, nil
}

// DiskUsageStatsAsString concatenates disk usage statistics into one string
func DiskUsageStatsAsString(disk DiskState, diskName string, threshold uint64, host string) string {
	statHeader := fmt.Sprintf("*WARNING!*\nLOW DISK SPACE ON `%s` \nMACHINE `%s`\n", diskName, host)
	statAll := fmt.Sprintf("TOTAL: %s\n", ByteSize(disk.All))
	statFree := fmt.Sprintf("FREE: %s\n", ByteSize(disk.Free))
	statUsed := fmt.Sprintf("USED: %s\n", ByteSize(disk.Used))
	statFreePerc := fmt.Sprintf("Free space in percentage: %d%%\n", disk.FreePercentage)
	statFooter := fmt.Sprintf("Using threshold %d%%", threshold)
	return statHeader + statAll + statFree + statUsed + statFreePerc + statFooter
}

// SendDiskSpaceReport compares current free disk space to threshold
func SendDiskSpaceReport(disk DiskState, threshold uint64, target string, wg *sync.WaitGroup) {
	api := slack.New(os.Getenv("SLACK_SECRET_KEY"))
	// Decrement WaitGroup counter
	defer wg.Done()
	params := slack.PostMessageParameters{}
	channelID, timestamp, err := api.PostMessage(target, DiskUsageStatsAsString(disk, disk.Name, threshold, disk.Host), params)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s - Message sent to %s\n", timestamp, channelID)
}

// MapStrToInt will map a atoi function to a slice
func MapStrToInt(strArray []string) []uint64 {
	intArray := make([]uint64, len(strArray))
	for i, v := range strArray {
		intValue, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			panic(err)
		}
		intArray[i] = intValue
	}
	return intArray
}

func main() {
	// Parse cmd args
	diskNamePtr := flag.String("disk", "/ /tmp", "Disk names as Strings, separated by space.")
	thresholdPtr := flag.String("threshold", "10 10", "Integers representing the maximum percentage of free space before alerting, seperated by spaces.")
	targetPtr := flag.String("target", "#target_slack_channel", "Target Person or Channel on Slack.")
	flag.Parse()

	diskNames := strings.Fields(*diskNamePtr)
	thresholdValuesStr := strings.Fields(*thresholdPtr)

	// Convert threshold values to integers
	thresholdValues := MapStrToInt(thresholdValuesStr)

	// Check if diskNames and thresholdValues contain the same amount of values
	if len(diskNames) != len(thresholdValues) {
		panic("-disk and -threshold arguments need to have same amount of values!")
	}

	// Create a map from diskNames and thresholdValues
	var diskData map[string]uint64
	diskData = make(map[string]uint64)
	for i, v := range diskNames {
		diskData[v] = thresholdValues[i]
	}

	// Create WaitGroup for async workflow
	var wg sync.WaitGroup
	for diskName, thresholdValue := range diskData {
		disk, err := StatDisk(diskName)
		if err != nil {
			panic(err)
		}
		if disk.FreePercentage < thresholdValue {
			// Increment the WaitGroup counter.
			wg.Add(1)
			go SendDiskSpaceReport(disk, thresholdValue, *targetPtr, &wg)
		}
	}
	// Wait for all Slack reports to be sent.
	wg.Wait()
}
