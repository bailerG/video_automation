package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Global variables to store data between function calls
var (
	config           Config
	currentStory     string
	currentAudioURL  string
	currentVideoURL  string
	currentMergedURL string
	topic            = "artificial intelligence" // Default topic
	httpClient       = &http.Client{Timeout: 30 * time.Second}
)

// Main function - entry point
func main() {
	fmt.Println("🎬 TikTok Video Creation Automation")
	fmt.Println("===================================")

	// Validate configuration
	if config.GeminiAPIKey == "your-gemini-api-key" {
		fmt.Println("⚠️  Please update the API keys in the config before running")
		fmt.Println("   Set your actual API keys in the init() function")
		return
	}

	// Set custom topic if needed
	// topic = "machine learning breakthroughs" // Uncomment to change topic

	// Start the automation flow
	Trigger()

	fmt.Println("\n✨ Automation flow completed!")
	fmt.Println("In a production environment, this would run every hour via scheduler.")
}

// Initialize configuration
func init() {
	config = Config{
		GeminiAPIKey:      getEnvOrDefault("GEMINI_API_KEY", "your-gemini-api-key"),
		TTSAPIKey:         getEnvOrDefault("TTS_API_KEY", "your-tts-api-key"),
		GoogleDriveAPIKey: getEnvOrDefault("GOOGLEDRIVE_API_KEY", "your-googledrive-api-key"),
		MergeServiceURL:   getEnvOrDefault("MERGE_SERVICE_URL", "https://your-merge-service.com"),
		RawVideoFileID:    getEnvOrDefault("RAW_VIDEO_FILE_ID", "YOUR_RAW_VIDEO_FILE_ID"),
		OutputFolderID:    getEnvOrDefault("OUTPUT_FOLDER_ID", "YOUR_OUTPUT_FOLDER_ID"),
	}
}

// Helper function to get environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// HTTP request helper function
func makeHTTPRequest(method, url string, headers map[string]string, body interface{}) ([]byte, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

// Trigger function - starts the automation flow
func Trigger() {
	// Starts a new generation every 3 hours
	for true {
		fmt.Println("🚀 Starting video creation automation flow...")
		fmt.Printf("⏰ Scheduled trigger activated at %s\n", time.Now().Format("2006-01-02 15:04:05"))

		GenStory()

		time.Sleep(3 * time.Hour)
	}
}

// GenStory function - generates TikTok script using Gemini
func GenStory() {
	fmt.Println("📝 Generating TikTok story...")

	prompt := fmt.Sprintf("Write a 60-second TikTok script that hooks viewers in the first 3 seconds and tells a compelling, shareable story about %s.", topic)

	requestBody := GeminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: struct {
			Temperature     float64 `json:"temperature"`
			MaxOutputTokens int     `json:"maxOutputTokens"`
		}{
			Temperature:     0.8,
			MaxOutputTokens: 250,
		},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro-latest:generateContent?key=%s", config.GeminiAPIKey)
	responseBody, err := makeHTTPRequest("POST", url, headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to generate story: %v", err)
		// In production, you might want to retry or use a fallback
		return
	}

	var response GeminiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse Gemini response: %v", err)
		return
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		log.Printf("❌ No story generated")
		return
	}

	currentStory = response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ Generated story: %s\n", currentStory[:100]+"...")
	QAStory()
}

// QAStory function - evaluates the story quality
func QAStory() {
	fmt.Println("🔍 Evaluating story quality...")

	prompt := fmt.Sprintf("Evaluate the following TikTok script for virality: %s. Score 1-10 and suggest improvements if under 8.", currentStory)

	requestBody := GeminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: struct {
			Temperature     float64 `json:"temperature"`
			MaxOutputTokens int     `json:"maxOutputTokens"`
		}{
			Temperature:     0.5,
			MaxOutputTokens: 150,
		},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro-latest:generateContent?key=%s", config.GeminiAPIKey)
	responseBody, err := makeHTTPRequest("POST", url, headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to evaluate story: %v", err)
		return
	}

	var response GeminiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse QA response: %v", err)
		return
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		log.Printf("❌ No QA response received")
		return
	}

	qaResponse := response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ QA Response: %s\n", qaResponse)

	// Check if score is 1-7 (needs improvement)
	if strings.Contains(qaResponse, "Score: 1") ||
		strings.Contains(qaResponse, "Score: 2") ||
		strings.Contains(qaResponse, "Score: 3") ||
		strings.Contains(qaResponse, "Score: 4") ||
		strings.Contains(qaResponse, "Score: 5") ||
		strings.Contains(qaResponse, "Score: 6") ||
		strings.Contains(qaResponse, "Score: 7") {
		fmt.Println("❌ Score too low, regenerating story...")
		GenStory() // Loop back if score is too low
		return
	}

	fmt.Println("✅ Story quality approved, proceeding to TTS...")
	MakeTTS()
}

// MakeTTS function - converts text to speech
func MakeTTS() {
	fmt.Println("🎤 Converting text to speech...")

	// Define the ElevenLabs API endpoint
	apiURL := "https://api.elevenlabs.io/v1/text-to-speech"

	// Construct the request body
	ttsRequest := map[string]interface{}{
		"text": currentStory,
		"voice_settings": map[string]interface{}{
			"stability":        0.75,
			"similarity_boost": 0.85,
		},
	}

	// Marshal the request body to JSON
	requestBody, err := json.Marshal(ttsRequest)
	if err != nil {
		log.Printf("❌ Failed to marshal TTS request: %v", err)
		return
	}

	// Set up headers
	headers := map[string]string{
		"Authorization": "Bearer " + config.TTSAPIKey,
		"Content-Type":  "application/json",
	}

	// Make the HTTP request
	responseBody, err := makeHTTPRequest("POST", apiURL, headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to convert text to speech: %v", err)
		return
	}

	// Parse the response
	var response TTSResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse TTS response: %v", err)
		return
	}

	currentAudioURL = response.AudioURL
	fmt.Printf("✅ TTS completed. Audio URL: %s\n", currentAudioURL)

	QAVO()
}

// QAVO function - quality assurance for voiceover
func QAVO() {
	fmt.Println("🎧 Checking voiceover quality...")

	prompt := fmt.Sprintf("Transcribe and check clarity of this voiceover. Return 'OK' or suggest fixes: %s", currentAudioURL)

	requestBody := GeminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: struct {
			Temperature     float64 `json:"temperature"`
			MaxOutputTokens int     `json:"maxOutputTokens"`
		}{
			Temperature:     0,
			MaxOutputTokens: 100,
		},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro-latest:generateContent?key=%s", config.GeminiAPIKey)
	responseBody, err := makeHTTPRequest("POST", url, headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to check audio quality: %v", err)
		return
	}

	var response GeminiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse audio QA response: %v", err)
		return
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		log.Printf("❌ No audio QA response received")
		return
	}

	qaResponse := response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ Audio QA Response: %s\n", qaResponse)

	// Check if response contains suggestions for fixes
	if strings.Contains(strings.ToLower(qaResponse), "suggest") {
		fmt.Println("❌ Audio quality issues detected, regenerating TTS...")
		MakeTTS() // Loop back if audio needs fixes
		return
	}

	fmt.Println("✅ Audio quality approved, fetching video...")
	FetchVideo()
}

// FetchVideo function - retrieves raw video from Google Drive
func FetchVideo() {
	fmt.Println("📹 Fetching raw video from Google Drive...")

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?fields=id,name,webContentLink&key=%s",
		config.RawVideoFileID, config.GoogleDriveAPIKey)

	headers := map[string]string{
		"Authorization": "Bearer " + config.GoogleDriveAPIKey,
	}

	responseBody, err := makeHTTPRequest("GET", url, headers, nil)
	if err != nil {
		log.Printf("❌ Failed to fetch video from Google Drive: %v", err)
		return
	}

	var response GoogleDriveResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse Google Drive response: %v", err)
		return
	}

	currentVideoURL = response.WebContentLink
	fmt.Printf("✅ Video fetched successfully: %s\n", currentVideoURL)
	MergeAV()
}

// MergeAV function - merges audio and video
func MergeAV() {
	fmt.Println("🔧 Merging audio and video...")

	requestBody := MergeRequest{
		VideoURL:    currentVideoURL,
		AudioURL:    currentAudioURL,
		StartOffset: 0,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + config.TTSAPIKey, // Assuming same API key
		"Content-Type":  "application/json",
	}

	responseBody, err := makeHTTPRequest("POST", config.MergeServiceURL+"/merge", headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to merge audio and video: %v", err)
		return
	}

	var response MergeResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse merge response: %v", err)
		return
	}

	currentMergedURL = response.OutputURL
	fmt.Printf("✅ Audio/Video merge completed: %s\n", currentMergedURL)
	FinalQA()
}

// FinalQA function - final quality assessment
func FinalQA() {
	fmt.Println("🎯 Performing final quality assessment...")

	prompt := fmt.Sprintf("Assess the final video here: %s. Check audio levels, pacing, and suggest if it meets TikTok viral standards.", currentMergedURL)

	requestBody := GeminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: struct {
			Temperature     float64 `json:"temperature"`
			MaxOutputTokens int     `json:"maxOutputTokens"`
		}{
			Temperature:     0.5,
			MaxOutputTokens: 200,
		},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro-latest:generateContent?key=%s", config.GeminiAPIKey)
	responseBody, err := makeHTTPRequest("POST", url, headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to perform final QA: %v", err)
		return
	}

	var response GeminiResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse final QA response: %v", err)
		return
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		log.Printf("❌ No final QA response received")
		return
	}

	qaResponse := response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ Final QA Response: %s\n", qaResponse)

	// Check if response contains suggestions (indicating issues)
	if strings.Contains(strings.ToLower(qaResponse), "suggest") {
		fmt.Println("❌ Final quality issues detected, restarting entire flow...")
		GenStory() // Restart whole flow if there are issues
		return
	}

	fmt.Println("✅ Final quality approved, saving video...")
	SaveFinal()
}

// SaveFinal function - saves the final video to Google Drive
func SaveFinal() {
	fmt.Println("💾 Saving final video to Google Drive...")

	fileName := fmt.Sprintf("tiktok_%s.mp4", time.Now().Format("20060102_150405"))

	requestBody := GoogleDriveUploadRequest{
		FolderID: config.OutputFolderID,
	}
	requestBody.File.URL = currentMergedURL
	requestBody.File.Name = fileName

	headers := map[string]string{
		"Authorization": "Bearer " + config.GoogleDriveAPIKey,
		"Content-Type":  "application/json",
	}

	responseBody, err := makeHTTPRequest("POST", "https://www.googleapis.com/upload/drive/v3/files", headers, requestBody)
	if err != nil {
		log.Printf("❌ Failed to save video to Google Drive: %v", err)
		return
	}

	var response GoogleDriveResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse Google Drive upload response: %v", err)
		return
	}

	fmt.Printf("✅ Video saved successfully! File ID: %s\n", response.ID)
	fmt.Printf("🎉 Video creation automation completed successfully!\n")
	fmt.Printf("📱 Ready for TikTok upload: %s\n", fileName)
}

// Config Configuration struct for API connections
type Config struct {
	GeminiAPIKey      string
	TTSAPIKey         string
	GoogleDriveAPIKey string
	MergeServiceURL   string
	RawVideoFileID    string
	OutputFolderID    string
}

// GeminiRequest Request structures for Gemini API calls
type GeminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		Temperature     float64 `json:"temperature"`
		MaxOutputTokens int     `json:"maxOutputTokens"`
	} `json:"generationConfig"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type TTSRequest struct {
	Voice        string `json:"voice"`
	Text         string `json:"text"`
	OutputFormat string `json:"output_format"`
}

type TTSResponse struct {
	AudioURL string `json:"audio_url"`
}

type GoogleDriveResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	WebContentLink string `json:"webContentLink"`
}

type MergeRequest struct {
	VideoURL    string `json:"video_url"`
	AudioURL    string `json:"audio_url"`
	StartOffset int    `json:"start_offset"`
}

type MergeResponse struct {
	OutputURL string `json:"output_url"`
}

type GoogleDriveUploadRequest struct {
	FolderID string `json:"folderId"`
	File     struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	} `json:"file"`
}
