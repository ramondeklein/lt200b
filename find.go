package main

import (
	"strings"
)

func isMACAddress(address string) bool {
	parts := strings.Split(strings.ReplaceAll(address, "-", ":"), ":")
	if len(parts) != 6 {
		return false
	}
	for _, part := range parts {
		if len(part) != 2 {
			return false
		}
		for _, c := range part {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// matchPrinter reports whether a scan result is the requested printer.
// On macOS, CoreBluetooth identifies peripherals by a UUID. The MAC printed
// on the LetraTag is only present at the end of the advertised name.
func matchPrinter(address, deviceAddress, localName string, macOS bool) bool {
	if macOS && isMACAddress(address) {
		suffix := compactHex(address)
		name := compactHex(localName)
		return strings.HasPrefix(name, "LETRATAG") && strings.HasSuffix(name, suffix)
	}
	return strings.EqualFold(deviceAddress, address)
}

func compactHex(value string) string {
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, ":", "")
	return strings.ToUpper(value)
}
