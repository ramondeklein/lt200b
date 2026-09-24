package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

func printJob(address string, chunks [][]byte) error {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return err
	}

	deviceAddress, err := findDevice(adapter, address)
	if err != nil {
		return err
	}
	device, err := adapter.Connect(deviceAddress, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}
	defer device.Disconnect()

	services, err := device.DiscoverServices(nil)
	if err != nil {
		return err
	}

	var printChar bluetooth.DeviceCharacteristic
	found := false
	for _, service := range services {
		if !strings.HasPrefix(strings.ToLower(service.UUID().String()), "be3dd650") {
			continue
		}
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			return err
		}
		for _, char := range chars {
			if strings.HasPrefix(strings.ToLower(char.UUID().String()), "be3dd651") {
				printChar = char
				found = true
			}
		}
	}
	if !found {
		return fmt.Errorf("LetraTag print service was not found")
	}

	for _, chunk := range chunks {
		if _, err := printChar.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

func findDevice(adapter *bluetooth.Adapter, address string) (bluetooth.Address, error) {
	macOS := runtime.GOOS == "darwin"
	found := make(chan bluetooth.Address, 1)
	var once sync.Once
	stop := func() {
		once.Do(func() { _ = adapter.StopScan() })
	}
	timer := time.AfterFunc(10*time.Second, stop)
	defer timer.Stop()

	err := adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
		if !matchPrinter(address, result.Address.String(), result.LocalName(), macOS) {
			return
		}
		select {
		case found <- result.Address:
		default:
		}
		stop()
	})

	select {
	case deviceAddress := <-found:
		return deviceAddress, nil
	default:
		if err != nil {
			return bluetooth.Address{}, err
		}
		return bluetooth.Address{}, fmt.Errorf("device with address %s was not found. Turn the LetraTag on and keep it nearby", address)
	}
}
