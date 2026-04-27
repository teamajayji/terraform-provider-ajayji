package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type AjayjiClient struct {
	Endpoint   string
	HttpClient *http.Client
}

type PersonaPayload struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	InputTopic   string `json:"input_topic,omitempty"`
	OutputTopic  string `json:"output_topic,omitempty"`
	InputScript  string `json:"input_script,omitempty"`
	OutputScript string `json:"output_script,omitempty"`
}

type HuggingFaceConfigPayload struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Token string `json:"token"`
}


func (c *AjayjiClient) CreatePersona(payload PersonaPayload) (*PersonaPayload, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/personas", c.Endpoint), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create persona, status code: %d", res.StatusCode)
	}

	var created PersonaPayload
	err = json.NewDecoder(res.Body).Decode(&created)
	return &created, err
}

func (c *AjayjiClient) GetPersona(id string) (*PersonaPayload, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/personas/%s", c.Endpoint, id), nil)
	if err != nil {
		return nil, err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, nil // Return nil if it was deleted so Terraform knows it drifted
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get persona, status code: %d", res.StatusCode)
	}

	var persona PersonaPayload
	err = json.NewDecoder(res.Body).Decode(&persona)
	return &persona, err
}

func (c *AjayjiClient) UpdatePersona(id string, payload PersonaPayload) (*PersonaPayload, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/personas/%s", c.Endpoint, id), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update persona, status code: %d", res.StatusCode)
	}

	var updated PersonaPayload
	err = json.NewDecoder(res.Body).Decode(&updated)
	return &updated, err
}

func (c *AjayjiClient) DeletePersona(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/personas/%s", c.Endpoint, id), nil)
	if err != nil {
		return err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete persona, status code: %d", res.StatusCode)
	}

	return nil
}


// handling models 
type ModelPayload struct {
	Repo     string `json:"repo"`
	FileName string `json:"fileName"`
	HuggingFaceConfigId string `json:"huggingface_config_id,omitempty"`
}

type ModelStatus struct {
	FileName string  `json:"fileName"`
	Repo     string  `json:"repo"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
}

type ModelsResponse struct {
	Models []ModelStatus `json:"models"`
}

func (c *AjayjiClient) PullModel(repo, fileName, configId string) error {
	payload := ModelPayload{
		Repo:                repo,
		FileName:            fileName,
		HuggingFaceConfigId: configId,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/pull", c.Endpoint), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pull model, status code: %d", res.StatusCode)
	}
	return nil
}

func (c *AjayjiClient) GetModel(fileName string) (*ModelStatus, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/models", c.Endpoint), nil)
	if err != nil {
		return nil, err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var response ModelsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	for _, m := range response.Models {
		if m.FileName == fileName {
			return &m, nil
		}
	}
	return nil, nil // Not found
}

func (c *AjayjiClient) DeleteModel(fileName string) error {
	payload := map[string]string{"fileName": fileName}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/rm", c.Endpoint), bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete model, status code: %d", res.StatusCode)
	}
	return nil
}

// --- Hugging Face Credentials ---

func (c *AjayjiClient) CreateHuggingFaceConfig(payload HuggingFaceConfigPayload) (*HuggingFaceConfigPayload, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/huggingface_configs", c.Endpoint), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create hf config, status code: %d", res.StatusCode)
	}

	var created HuggingFaceConfigPayload
	err = json.NewDecoder(res.Body).Decode(&created)
	return &created, err
}

func (c *AjayjiClient) GetHuggingFaceConfig(id string) (*HuggingFaceConfigPayload, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/huggingface_configs/%s", c.Endpoint, id), nil)
	if err != nil {
		return nil, err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, nil // Drift detection
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get hf config, status code: %d", res.StatusCode)
	}

	var config HuggingFaceConfigPayload
	err = json.NewDecoder(res.Body).Decode(&config)
	return &config, err
}

func (c *AjayjiClient) UpdateHuggingFaceConfig(id string, payload HuggingFaceConfigPayload) (*HuggingFaceConfigPayload, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/huggingface_configs/%s", c.Endpoint, id), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update hf config, status code: %d", res.StatusCode)
	}

	var updated HuggingFaceConfigPayload
	err = json.NewDecoder(res.Body).Decode(&updated)
	return &updated, err
}

func (c *AjayjiClient) DeleteHuggingFaceConfig(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/huggingface_configs/%s", c.Endpoint, id), nil)
	if err != nil {
		return err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete hf config, status code: %d", res.StatusCode)
	}

	return nil
}