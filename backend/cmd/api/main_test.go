package main

import "testing"

func TestCheckLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:8080": true, "localhost:8080": true, "[::1]:8080": true,
		"0.0.0.0:8080": false, ":8080": false, "192.168.0.10:8080": false, "nada": false,
	} {
		if err := checkLoopback(addr); (err == nil) != ok {
			t.Errorf("checkLoopback(%q) = %v", addr, err)
		}
	}
}
