package operator

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var Endpoint = "https://rtctunnel-operator.fly.dev"

func Pull(key string) (string, error) {
	uv := url.Values{
		"address": {key},
	}
	for {
		req, _ := http.NewRequest("POST", Endpoint+"/sub", strings.NewReader(uv.Encode()))
		req.Header.Set("js.fetch:mode", "cors")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				log.Print("[operator] timed-out, retrying")
				continue
			}
			return "", err
		}
		if resp.StatusCode == http.StatusGatewayTimeout {
			log.Print("[operator] timed-out, retrying")
			resp.Body.Close()
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", errors.New(resp.Status)
		}

		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(bs), nil
	}
}

func Push(key, data string) error {
	uv := url.Values{
		"address": {key},
		"data":    {data},
	}
	req, _ := http.NewRequest("POST", Endpoint+"/pub", strings.NewReader(uv.Encode()))
	req.Header.Set("js.fetch:mode", "cors")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		log.Println("push failed: ", resp.Status)
		return fmt.Errorf("push failed: %s", resp.Status)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return nil
}
