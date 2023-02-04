package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	numOfRetries  *int
	numOfStreams  *int
	path          *string
	resultsQueues []chan Result
	currentQueue  = 0
)

func init() {
	path = flag.String("p", "./data", "path to save")
	numOfStreams = flag.Int("n", 30, "number of parallel streams to download")
	numOfRetries = flag.Int("r", 5, "number of http retries")
}

func getIsicResult(url string, result *Response) {
	client := http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if err := json.Unmarshal(body, result); err != nil { // Parse []byte to the go struct pointer
		log.Fatalln(err)
	}
}

func processResponse(result Response) {
	for _, r := range result.Results {
		resultsQueues[currentQueue] <- r
		currentQueue++
		if currentQueue == *numOfStreams {
			currentQueue = 0
		}
	}
}

func downloadFile(url, path, fileName string) error {
	retries := *numOfRetries
	for retries > 0 {
		client := http.Client{
			Timeout: 60 * time.Second,
		}
		resp, err := client.Get(url)
		if err != nil {
			log.Println(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Println("received non 200 response code")
			retries -= 1
		} else {

			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				err := os.MkdirAll(path, os.ModePerm)
				if err != nil {
					return err
				}
			}

			file, err := os.Create(fmt.Sprintf("%v/%v", path, fileName))
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(file, resp.Body)
			if err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func main() {
	flag.Parse()

	resultsQueues = make([]chan Result, *numOfStreams)

	for i := 0; i < *numOfStreams; i++ {
		resultsQueues[i] = make(chan Result, 10000)
	}
	var result Response
	getIsicResult("https://api.isic-archive.com/api/v2/images/?limit=100", &result)
	bar := progressbar.Default(result.Count)

	if _, err := os.Stat(*path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(*path, os.ModePerm)
		if err != nil {
			log.Fatalln("failed to create a dir", err)
		}
	}

	file, err := os.Create(fmt.Sprintf("%v/dataset.csv", *path))
	defer file.Close()
	if err != nil {
		log.Fatalln("failed to open file", err)
	}

	w := csv.NewWriter(file)
	defer w.Flush()

	for i := 0; i < *numOfStreams; i++ {
		i := i
		go func() {
			for {
				select {
				case r := <-resultsQueues[i]:
					diagnosis := r.Metadata.Clinical.Diagnosis
					id := r.IsicId

					err := downloadFile(r.Files.Thumbnail.Url, fmt.Sprintf("%v/%v", *path, diagnosis),
						fmt.Sprintf("%v.jpg", id))
					if err != nil {
						log.Fatalln(err)
					}

					row := []string{id, diagnosis}
					if err := w.Write(row); err != nil {
						log.Fatalln("error writing record to file", err)
					}

					err = bar.Add(1)
					if err != nil {
						log.Fatalln("failed with progress bar", err)
					}
				}
			}
		}()
	}

	processResponse(result)
	for result.Next != nil {
		getIsicResult(*result.Next, &result)
		processResponse(result)
		time.Sleep(40 * time.Millisecond)
	}

	for {
		sum := 0
		for i := 0; i < *numOfStreams; i++ {
			sum += len(resultsQueues[i])
		}
		time.Sleep(1 * time.Second)
		if sum == 0 {
			break
		}
	}
	println("done!")
}
