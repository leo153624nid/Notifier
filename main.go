package main

import "fmt"

const appVersion = "1.0.0"

func main() {
	recipient := "user@example.com"
	subject := "Deploy done"
	retryCount := 3
	timeout := 3.5 // float64
	isUrgent := false

	fmt.Println(recipient, subject, retryCount, timeout, isUrgent)
	fmt.Println(appVersion)
}
