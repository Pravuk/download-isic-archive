package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Example struct {
	Id        string
	Diagnosis string
}

type File struct {
	Url  string `json:"url"`
	Size int    `json:"size"`
}

type Response struct {
	Count   int64   `json:"count"`
	Next    *string `json:"next"`
	Results []struct {
		IsicId   string `json:"isic_id"`
		Metadata struct {
			Clinical struct {
				Diagnosis string `json:"diagnosis"`
			} `json:"clinical"`
		} `json:"metadata"`
		Files struct {
			Full      File `json:"full"`
			Thumbnail File `json:"thumbnail_256"`
		} `json:"files"`
	} `json:"results"`
}

var dataToSave []Example

func getIsicResult(url string, result *Response) {
	resp, err := http.Get(url)
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

func main() {
	var result Response
	getIsicResult("https://api.isic-archive.com/api/v2/images/?limit=100", &result)
	bar := progressbar.Default(result.Count)
	processResponse(result)
	for result.Next != nil {
		getIsicResult(*result.Next, &result)
		processResponse(result)
		bar.Add(1)
		time.Sleep(40 * time.Millisecond)
	}

	if _, err := os.Stat("./data"); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir("./data", os.ModePerm)
		if err != nil {
			log.Fatalln("failed to create a dir", err)
		}
	}
	file, err := os.Create("./data/dataset.csv")
	defer file.Close()
	if err != nil {
		log.Fatalln("failed to open file", err)
	}

	w := csv.NewWriter(file)
	defer w.Flush()

	for _, record := range dataToSave {
		row := []string{record.Id, record.Diagnosis}
		if err := w.Write(row); err != nil {
			log.Fatalln("error writing record to file", err)
		}
	}

	println("done!")
}

func processResponse(result Response) {
	for _, r := range result.Results {
		id := r.IsicId
		dataToSave = append(dataToSave, Example{
			Id:        id,
			Diagnosis: r.Metadata.Clinical.Diagnosis,
		})
		diagnosis := r.Metadata.Clinical.Diagnosis
		go func() {
			err := downloadFile(r.Files.Thumbnail.Url, fmt.Sprintf("./data/%v", diagnosis),
				fmt.Sprintf("%v.jpg", id))
			if err != nil {
				log.Fatalln(err)
			}
		}()
	}
}

func downloadFile(URL, path, fileName string) error {
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("received non 200 response code")
	}

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

	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}
