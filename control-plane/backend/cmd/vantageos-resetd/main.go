package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	gpio := getEnvInt("VANTAGEOS_RESET_GPIO", 17)
	pollMs := getEnvInt("VANTAGEOS_POLL_MS", 25)
	debounceMs := getEnvInt("VANTAGEOS_DEBOUNCE_MS", 80)
	backendURL := getEnv("VANTAGEOS_BACKEND_URL", "http://127.0.0.1:5000")
	pressURL := backendURL + "/api/v1/recovery/press"

	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", gpio)

	if _, err := os.Stat(valuePath); os.IsNotExist(err) {
		if err := writeFile("/sys/class/gpio/export", strconv.Itoa(gpio)); err != nil {
			log.Fatalf("export gpio %d: %v", gpio, err)
		}
		writeFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", gpio), "in")
		time.Sleep(100 * time.Millisecond)
	}

	debounceDur := time.Duration(debounceMs) * time.Millisecond
	pollDur := time.Duration(pollMs) * time.Millisecond

	var lastStableValue int = -1
	var lastPressTime time.Time
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		raw, err := os.ReadFile(valuePath)
		if err != nil {
			log.Printf("read gpio %d: %v", gpio, err)
			time.Sleep(pollDur)
			continue
		}

		val := strings.TrimSpace(string(raw))
		currentValue := 0
		if val == "0" {
			currentValue = 0
		} else {
			currentValue = 1
		}

		if lastStableValue == -1 {
			lastStableValue = currentValue
			time.Sleep(pollDur)
			continue
		}

		if currentValue != lastStableValue {
			sameCount := 0
			checkStart := time.Now()
			for time.Since(checkStart) < debounceDur {
				time.Sleep(pollDur)
				raw2, err := os.ReadFile(valuePath)
				if err != nil {
					break
				}
				newVal := strings.TrimSpace(string(raw2))
				newInt := 0
				if newVal != "0" {
					newInt = 1
				}
				if newInt == currentValue {
					sameCount++
				} else {
					sameCount = 0
					currentValue = newInt
				}
				if sameCount >= 2 {
					break
				}
			}

			if sameCount >= 2 {
				lastStableValue = currentValue
				if currentValue == 0 {
					now := time.Now()
					if lastPressTime.IsZero() || now.Sub(lastPressTime) > debounceDur*2 {
						lastPressTime = now
						body, _ := json.Marshal(map[string]any{})
						resp, err := client.Post(pressURL, "application/json", bytes.NewReader(body))
						if err != nil {
							log.Printf("post press event: %v", err)
						} else {
							resp.Body.Close()
							log.Printf("press event sent (gpio %d)", gpio)
						}
					}
				}
			}
		}

		time.Sleep(pollDur)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o644)
}
