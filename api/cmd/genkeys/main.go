package main

import (
	"fmt"
	"os"

	"github.com/SherClockHolmes/webpush-go"
)

func main() {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate VAPID keys: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("VAPID Public Key:  %s\n", publicKey)
	fmt.Printf("VAPID Private Key: %s\n", privateKey)
}
