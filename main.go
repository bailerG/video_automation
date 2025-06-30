package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
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
	if err := Trigger(); err != nil {
		log.Printf("Error running automation: %v", err)
		return
	}

	fmt.Println("\n✨ Automation flow completed!")
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
		switch b := body.(type) {
		case []byte:
			reqBody = bytes.NewBuffer(b)
		case string:
			reqBody = strings.NewReader(b)
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %v", err)
			}
			reqBody = bytes.NewBuffer(jsonBody)
		}
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

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

// getDriveService returns an authenticated Google Drive service
func getDriveService() (*drive.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err2 := saveToken(tokFile, tok); err2 != nil {
			return nil, err2
		}
	}

	client := config.Client(ctx, tok)
	return drive.NewService(ctx, option.WithHTTPClient(client))
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// getTokenFromWeb requests a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	url := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", url)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

// saveToken saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("unable to cache oauth token: %v", err)
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(token); err != nil {
		return err
	}
	return nil
}

// Trigger function - starts the automation flow
func Trigger() error {
	// Starts a new generation every 3 hours
	for true {
		fmt.Println("🚀 Starting video creation automation flow...")
		fmt.Printf("⏰ Scheduled trigger activated at %s\n", time.Now().Format("2006-01-02 15:04:05"))

		if err := GenStory(); err != nil {
			return err
		}

		time.Sleep(3 * time.Hour)
	}

	return nil
}

// GenStory function - generates TikTok script using Gemini
func GenStory() error {
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
		return fmt.Errorf("failed to generate story: %v", err)
	}

	var response GeminiResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse Gemini response: %v", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no story generated")
	}

	currentStory = response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ Generated story: %s\n", currentStory[:100]+"...")
	return QAStory(currentStory)
}

// QAStory function - evaluates the story quality
func QAStory(currentStory string) error {
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
		return fmt.Errorf("failed to evaluate story: %v", err)
	}

	var response GeminiResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse QA response: %v", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no QA response received")
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
		if err = GenStory(); err != nil {
			return fmt.Errorf("failed to regenerate story: %v", err)
		}
		return nil
	}

	fmt.Println("✅ Story quality approved, proceeding to TTS...")
	return MakeTTS(currentStory)
}

// MakeTTS function - converts text to speech
func MakeTTS(currentStory string) error {
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
		return fmt.Errorf("failed to marshal TTS request: %v", err)
	}

	// Set up headers
	headers := map[string]string{
		"Authorization": "Bearer " + config.TTSAPIKey,
		"Content-Type":  "application/json",
	}

	// Make the HTTP request
	responseBody, err := makeHTTPRequest("POST", apiURL, headers, requestBody)
	if err != nil {
		return fmt.Errorf("failed to convert text to speech: %v", err)
	}

	// Parse the response
	var response TTSResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse TTS response: %v", err)
	}

	fmt.Printf("✅ TTS completed.")
	if false {
		QAVO()
	}
	return FetchVideo(response.Audio)
}

// QAVO function - quality assurance for voiceover
func QAVO() error {
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
		return err
	}

	var response GeminiResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		log.Printf("❌ Failed to parse audio QA response: %v", err)
		return err
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		log.Printf("❌ No audio QA response received")
		return errors.New("no audio QA response received")
	}

	qaResponse := response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ Audio QA Response: %s\n", qaResponse)

	// Check if response contains suggestions for fixes
	if strings.Contains(strings.ToLower(qaResponse), "suggest") {
		fmt.Println("❌ Audio quality issues detected, regenerating TTS...")
		if err = MakeTTS(currentStory); err != nil {
			return err
		} // Loop back if audio needs fixes
		return nil
	}

	fmt.Println("✅ Audio quality approved, fetching video...")
	return FetchVideo(nil)
}

// FetchVideo function - retrieves raw video from Google Drive using Drive API
func FetchVideo(audioFile []byte) error {
	fmt.Println("📹 Fetching raw video from Google Drive (using Drive API)...")

	driveService, err := getDriveService()
	if err != nil {
		return fmt.Errorf("failed to create Drive service: %v", err)
	}

	file, err := driveService.Files.Get(config.RawVideoFileID).Fields("id", "name", "webContentLink").Do()
	if err != nil {
		return fmt.Errorf("failed to fetch video from Google Drive: %v", err)
	}

	videoFile, err := file.VideoMediaMetadata.MarshalJSON()
	fmt.Printf("✅ Video fetched successfully")
	return MergeAV(audioFile, videoFile)
}

// MergeAV function - merges audio and video
func MergeAV(audioFile []byte, videoFile []byte) error {
	fmt.Println("🔧 Merging audio and video...")

	requestBody := MergeRequest{
		Video:       audioFile,
		Audio:       videoFile,
		StartOffset: 0,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + config.TTSAPIKey, // Assuming same API key
		"Content-Type":  "application/json",
	}

	responseBody, err := makeHTTPRequest("POST", config.MergeServiceURL+"/merge", headers, requestBody)
	if err != nil {
		return fmt.Errorf("failed to merge audio and video: %v", err)
	}

	var response MergeResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse merge response: %v", err)
	}

	fmt.Printf("✅ Audio/Video merge completed: %s\n", response.OutputURL)
	return FinalQA(response.OutputURL)
}

// FinalQA function - final quality assessment
func FinalQA(finalVideoURL string) error {
	fmt.Println("🎯 Performing final quality assessment...")

	prompt := fmt.Sprintf("Assess the final video here: %s. Check audio levels, pacing, and suggest if it meets TikTok viral standards.", finalVideoURL)

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
		return fmt.Errorf("failed to perform final QA: %v", err)
	}

	var response GeminiResponse
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse final QA response: %v", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no final QA response received")
	}

	qaResponse := response.Candidates[0].Content.Parts[0].Text
	fmt.Printf("✅ Final QA Response: %s\n", qaResponse)

	// Check if response contains suggestions (indicating issues)
	if strings.Contains(strings.ToLower(qaResponse), "suggest") {
		fmt.Println("❌ Final quality issues detected, restarting entire flow...")
		if err = GenStory(); err != nil {
			return err
		} // Restart whole flow if there are issues
		return nil
	}

	fmt.Println("✅ Final quality approved, saving video...")
	return SaveFinal(finalVideoURL)
}

// SaveFinal function - saves the final video to Google Drive using Drive API
func SaveFinal(finalVideoURL string) error {
	fmt.Println("💾 Saving final video to Google Drive (using Drive API)...")

	fileName := fmt.Sprintf("tiktok_%s.mp4", time.Now().Format("20060102_150405"))

	driveService, err := getDriveService()
	if err != nil {
		return fmt.Errorf("failed to create Drive service: %v", err)
	}

	// Download the merged video file
	resp, err := http.Get(finalVideoURL)
	if err != nil {
		return fmt.Errorf("failed to download merged video: %v", err)
	}
	defer resp.Body.Close()

	fileMetadata := &drive.File{
		Name:     fileName,
		Parents:  []string{config.OutputFolderID},
		MimeType: "video/mp4",
	}

	driveFile, err := driveService.Files.Create(fileMetadata).Media(resp.Body).Do()
	if err != nil {
		return fmt.Errorf("failed to upload video to Google Drive: %v", err)
	}

	fmt.Printf("✅ Video saved successfully! File name: %s\n", driveFile.Name)
	fmt.Printf("🎉 Video creation automation completed successfully!\n")
	fmt.Printf("📱 Ready for TikTok upload: %s\n", fileName)
	return nil
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
	Audio []byte `json:"file"`
}

type GoogleDriveResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	WebContentLink string `json:"webContentLink"`
	VideoBytes     []byte `json:"videoBytes"`
}

type MergeRequest struct {
	Video       []byte `json:"video_url"`
	Audio       []byte `json:"audio_url"`
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
