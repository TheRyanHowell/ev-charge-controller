package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	if p := os.Getenv("POWER_WATTS"); p != "" {
		if _, err := fmt.Sscanf(p, "%f", &defaultHandler.maxPowerWatts); err != nil {
			log.Fatalf("Invalid POWER_WATTS value: %v", err)
		}
	} else {
		log.Fatal("POWER_WATTS environment variable is required")
	}
	if v := os.Getenv("VOLTAGE"); v != "" {
		if _, err := fmt.Sscanf(v, "%f", &defaultHandler.voltage); err != nil {
			log.Printf("Invalid VOLTAGE value, using default 230V: %v", err)
			defaultHandler.voltage = 230.0
		}
	}
	if f := os.Getenv("FREQUENCY"); f != "" {
		if _, err := fmt.Sscanf(f, "%f", &defaultHandler.frequency); err != nil {
			log.Printf("Invalid FREQUENCY value, using default 50Hz: %v", err)
			defaultHandler.frequency = 50.0
		}
	}
	defaultHandler.energyData.Freq = defaultHandler.frequency
	defaultHandler.energyData.Voltage = defaultHandler.voltage
	defaultHandler.authUsername = os.Getenv("USERNAME")
	defaultHandler.authPassword = os.Getenv("PASSWORD")

	stateDir := os.Getenv("STATE_DIR")
	if stateDir == "" {
		stateDir = "/data"
	}
	defaultHandler.mqttStateFile = fmt.Sprintf("%s/mqtt-state-%s.json", stateDir, port)

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Printf("mqtt: failed to create state directory %s: %v", stateDir, err)
	}

	if defaultHandler.loadMQTTConfig() {
		go defaultHandler.reconnectMQTT()
	}

	mux := http.NewServeMux()
	mux.Handle("/cm", defaultHandler)
	mux.HandleFunc("/", defaultHandler.handleRoot)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		defaultHandler.Reset()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		defaultHandler.mu.RLock()
		defer defaultHandler.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"power":  defaultHandler.powerState,
			"energy": defaultHandler.energyData,
		})
	})

	log.Printf("Mock Tasmota server starting on :%s", port)
	log.Printf("Configuration: POWER=%.0f, VOLTAGE=%.0f, FREQUENCY=%.1f",
		defaultHandler.maxPowerWatts, defaultHandler.voltage, defaultHandler.frequency)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
