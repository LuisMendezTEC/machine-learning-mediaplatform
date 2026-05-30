package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	dir := flag.String("dir", "dataset/files", "directory with files to upload")
	name := flag.String("name", "Case from case_client", "case name")
	priority := flag.Int("priority", 5, "case priority 1-10")
	coord := flag.String("coordinator", "http://localhost:8080", "coordinator base URL")
	flag.Parse()

	files, err := os.ReadDir(*dir)
	if err != nil {
		log.Fatalf("read dir: %v", err)
	}

	var uploaded []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fp := filepath.Join(*dir, f.Name())
		path, err := uploadFile(*coord, fp)
		if err != nil {
			log.Printf("upload failed %s: %v", fp, err)
			continue
		}
		uploaded = append(uploaded, path)
		log.Printf("uploaded: %s -> %s", fp, path)
	}

	if len(uploaded) == 0 {
		log.Fatalf("no files uploaded, aborting")
	}

	caseID, err := createCase(*coord, *name, "Created by case_client", *priority, uploaded)
	if err != nil {
		log.Fatalf("create case: %v", err)
	}
	fmt.Printf("Case created: %s\n", caseID)

	// Poll until case completed
	for {
		c, err := getCase(*coord, caseID)
		if err != nil {
			log.Printf("get case: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		fmt.Printf("case %s status=%s risk_score=%.2f\n", caseID, c.Status, c.RiskScore)
		if c.Status == "completed" || c.Status == "failed" {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// Fetch report
	rep, err := getReport(*coord, caseID)
	if err != nil {
		log.Fatalf("get report: %v", err)
	}
	out, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Printf("Report:\n%s\n", string(out))
}

func uploadFile(coord, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", err
	}
	w.Close()

	url := coord + "/upload"
	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload returned %d", resp.StatusCode)
	}
	var out struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Path, nil
}

func createCase(coord, name, desc string, priority int, files []string) (string, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": desc,
		"priority":    priority,
		"files":       files,
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(coord+"/cases", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create case returned %d", resp.StatusCode)
	}
	var out struct {
		Case struct {
			ID string `json:"id"`
		} `json:"case"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Case.ID, nil
}

type CaseResp struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	RiskScore float64 `json:"risk_score"`
}

func getCase(coord, id string) (*CaseResp, error) {
	resp, err := http.Get(coord + "/cases/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get case returned %d", resp.StatusCode)
	}
	var out struct {
		Case CaseResp `json:"case"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out.Case, nil
}

func getReport(coord, id string) (map[string]interface{}, error) {
	resp, err := http.Get(coord + "/cases/" + id + "/report")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("report returned %d", resp.StatusCode)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
