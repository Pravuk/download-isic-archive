package main

type File struct {
	Url  string `json:"url"`
	Size int    `json:"size"`
}

type Result struct {
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
}

type Response struct {
	Count   int64    `json:"count"`
	Next    *string  `json:"next"`
	Results []Result `json:"results"`
}
