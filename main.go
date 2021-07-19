package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/slack-go/slack"
)

type Config struct {
	Slack     SlackConfig       `json:"slack"`
	Endpoints map[string]string `json:"endpoints"`
}

type SlackConfig struct {
	Channel string `json:"channel"`
	Token   string `json:"token"`
}

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

func readConfig(path string) (*Config, error) {

	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var config Config

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(byteValue, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func run() error {
	if !connected() {
		fmt.Println("You are not connected to the internet")
		return fmt.Errorf("You are not connected to the internet")
	}

	config, err := readConfig("healthcheck_config.json")
	if err != nil {
		return err
	}
	failed := make(chan string)
	errorChan := make(chan error, 1)

	var wg sync.WaitGroup

	for name, url := range config.Endpoints {
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

		var finalErr error

		if len(failedArr) > 0 {
			failedJoined := strings.Join(failedArr, ", ")

			text := fmt.Sprintf("Failed services: %s\n", failedJoined)

			fmt.Println(text)

			if config.Slack.Channel != "" && config.Slack.Token != "" {
				api := slack.New(config.Slack.Token)

				headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "❗ Major healthcheck failure", false, false))
				textBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil)

				textOption := slack.MsgOptionText("❗ Healthcheck failure!", false)

				msgBlocks := slack.MsgOptionBlocks(
					headerBlock,
					textBlock,
				)

				_, _, err := api.PostMessage(config.Slack.Channel, textOption, msgBlocks)
				if err != nil {
					finalErr = err
				}
			}

		} else {
			fmt.Println("OK!")
		}

		errorChan <- finalErr
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
