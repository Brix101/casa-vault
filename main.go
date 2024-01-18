package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxSize         = 10 * 1024 * 1024 * 1024 // 10 GB
	uploadDirectory = "uploads"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodPost:
		// Set the maximum file size to 10GB (in bytes)
		err := r.ParseMultipartForm(maxSize) // 10 GB
		if err != nil {
			log.Println("error parsing form", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.MkdirAll(uploadDirectory, 0755); err != nil {
			log.Println("error creating upload directory", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var wg sync.WaitGroup
		uploadErrors := make(chan error, len(r.MultipartForm.File))

		for _, fileHeaders := range r.MultipartForm.File {
			for _, fileHeader := range fileHeaders {
				wg.Add(1)
				go func(fileHeader *multipart.FileHeader) {
					defer wg.Done()
					file, err := fileHeader.Open()
					if err != nil {
						log.Println("Error opening file:", err)
						uploadErrors <- err
						return
					}
					defer file.Close()

					fileDst := filepath.Join(uploadDirectory, filepath.Clean(fileHeader.Filename))
					dst, err := os.Create(fileDst)
					if err != nil {
						log.Println("Error creating destination file:", err)
						uploadErrors <- err
						return
					}
					defer dst.Close()

					buf := make([]byte, 8*1024) // 8 KB buffer size

					if _, err := io.CopyBuffer(dst, file, buf); err != nil {
						log.Println("Error copying file:", err)
						uploadErrors <- err
						return
					}

					if err := os.Chmod(fileDst, 0644); err != nil {
						log.Println("Error setting file permissions:", err)
					}

					uploadErrors <- nil
					fmt.Fprintf(w, "Uploaded file: %s\n", fileHeader.Filename)

				}(fileHeader)
			}
		}

		go func() {
			wg.Wait()
			close(uploadErrors)
		}()

		for err := range uploadErrors {
			if err != nil {
				log.Println("error during file upload", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		fmt.Fprintf(w, "all files uploaded successfully")
	}
}

func main() {
	mux := http.NewServeMux()

	// Register your custom handler
	mux.HandleFunc("/upload", uploadHandler)
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDirectory))))

	// Use the custom multiplexer with http.ListenAndServe
	http.ListenAndServe("0.0.0.0:5000", mux)
}
