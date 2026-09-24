package main

import "testing"

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("LT200B_ADDRESS", "aa:bb:cc:dd:ee:ff")
	t.Setenv("LT200B_LISTEN", ":9090")

	address, listen := configFromEnv("", "")
	if address != "aa:bb:cc:dd:ee:ff" || listen != ":9090" {
		t.Fatalf("env config = %s %s", address, listen)
	}

	address, listen = configFromEnv("11:22:33:44:55:66", ":1")
	if address != "11:22:33:44:55:66" || listen != ":1" {
		t.Fatalf("explicit config = %s %s", address, listen)
	}
}
