package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <pods.txt>\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func parsePods(fn string) ([]string, error) {
	var pods []string

	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("error opening pods file %s: %w", fn, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		pods = append(pods, strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading pods file %s: %w", fn, err)
	}
	return pods, nil
}

func request(method, url string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	if headers == nil {
		headers = make(http.Header)
	}

	// Set a default User-Agent (if none set)
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", "./tools/check_pod_versions.go (https://git.mills.io/yarnsocial/yarn)")
	}

	req.Header = headers

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func makeJsonRequest(url string) ([]byte, error) {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")

	res, err := request(http.MethodGet, url, headers)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("non-success HTTP %s response for %s", res.Status, url)
	}

	if ctype := res.Header.Get("Content-Type"); ctype != "" {
		mediaType, _, err := mime.ParseMediaType(ctype)
		if err != nil {
			return nil, err
		}
		if mediaType != "application/json" {
			return nil, fmt.Errorf("non-JSON response content type '%s' for %s", ctype, url)
		}
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getLegacyPodVersion(url string) (string, error) {
	version := struct {
		FullVersion string
	}{
		FullVersion: "",
	}
	data, err := makeJsonRequest(url + "/version")
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return "", err
	}
	return version.FullVersion, nil
}

func getPodVersion(url string) (string, error) {
	info := struct {
		SoftwareVersion string `json:"software_version"`
	}{
		SoftwareVersion: "",
	}
	data, err := makeJsonRequest(url + "/info")
	if err != nil {
		return getLegacyPodVersion(url)
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return getLegacyPodVersion(url)
	}
	return info.SoftwareVersion, nil
}

type Pod struct {
	Name    string
	Version string
}

type Pods []Pod

func (pods Pods) Len() int           { return len(pods) }
func (pods Pods) Less(i, j int) bool { return strings.Compare(pods[i].Version, pods[j].Version) < 0 }
func (pods Pods) Swap(i, j int)      { pods[i], pods[j] = pods[j], pods[i] }

func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	knownPods, err := parsePods(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing pods.txt file %s: %s", flag.Arg(0), err)
		os.Exit(2)
	}

	results := make(chan Pod, len(knownPods))
	wg := sync.WaitGroup{}
	wg.Add(len(knownPods))
	for _, domain := range knownPods {
		url := fmt.Sprintf("https://%s", domain)
		go func(domain, url string) {
			defer wg.Done()
			pod := Pod{Name: domain}
			version, err := getPodVersion(url)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting pod version for %s: %s\n", pod.Name, err)
				pod.Version = "???"
			} else {
				pod.Version = version
			}
			results <- pod
		}(domain, url)
	}

	wg.Wait()
	close(results)

	var pods Pods

	for result := range results {
		pods = append(pods, result)
	}
	sort.Sort(pods)

	fmt.Println("Pod Version")
	for _, pod := range pods {
		fmt.Printf("%s %s\n", pod.Name, pod.Version)
	}
}
