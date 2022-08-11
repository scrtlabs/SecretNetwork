package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
)

var gramineServerURL string = "http://127.0.0.1:9005/"

const (
	CheckEnclave string = "check-enclave"
)

func getFromGramine(command string) (string, error) {
	var gramineResponse string
	var gramineError error

	requestURL := gramineServerURL + command
	var wg sync.WaitGroup
	wg.Add(1)
	go func(url string) {

		defer wg.Done()

		gramineReponseBody, er := doReq(url)
		gramineResponse = gramineReponseBody
		gramineError = er

	}(requestURL)
	wg.Wait()
	return gramineResponse, gramineError
}

func doReq(url string) (content string, err error) {

	response, err := http.Get(url)

	if err != nil {

		fmt.Errorf("%s", err)
		return "", err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)

	if err != nil {

		fmt.Errorf("%s", err)
		return "", err
	}

	return string(body), nil
}
