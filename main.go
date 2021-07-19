package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

func connected() (ok bool) {
	_, err := http.Get("http://clients3.google.com/generate_204")
	if err != nil {
		return false
	}
	return true
}

func isHealthy(url string) (ok bool, err error) {
	response, err := http.Get(url)
	if err != nil {
		return false, err
	}
	if response.StatusCode < 200 || response.StatusCode > 204 {
		return false, nil
	}
	return true, nil
}

func readEndpoints(path string) (map[string]string, error) {

	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var data map[string]string

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(byteValue, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func run() error {
	if !connected() {
		fmt.Println("You are not connected to the internet")
		return fmt.Errorf("You are not connected to the internet")
	}

	endpoints, err := readEndpoints("endpoints_config.json")
	if err != nil {
		return err
	}
	failed := make(chan string)
	errorChan := make(chan error, 1)

	var wg sync.WaitGroup

	for name, url := range endpoints {
		wg.Add(1)
		go func(name string, url string, wg *sync.WaitGroup) {
			defer wg.Done()
			healthy, err := isHealthy(url)
			if err != nil {
				errorChan <- err
			}
			if !healthy {
				failed <- name
			}
		}(name, url, &wg)
	}

	go func() {
		wg.Wait()
		close(failed)
	}()

	go func() {
		failedArr := make([]string, 0)
		for name := range failed {
			failedArr = append(failedArr, name)
		}

		if len(failedArr) > 0 {
			failedJoined := strings.Join(failedArr, ", ")
			fmt.Printf("Failed: %s\n", failedJoined)
		} else {
			fmt.Println("OK!")
		}

		errorChan <- nil
	}()

	err = <-errorChan
	if err != nil {
		return err
	}
	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Println("Failure", err)
		os.Exit(1)
	}
}
