package runway

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/igolaizola/vidai/internal/ratelimit"
)

type Client struct {
	client    *http.Client
	debug     bool
	ratelimit ratelimit.Lock
	token     string
	teamID    int
}

type Config struct {
	Token  string
	Wait   time.Duration
	Debug  bool
	Client *http.Client
}

func New(cfg *Config) *Client {
	wait := cfg.Wait
	if wait == 0 {
		wait = 1 * time.Second
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{
			Timeout: 2 * time.Minute,
		}
	}
	return &Client{
		client:    client,
		ratelimit: ratelimit.New(wait),
		debug:     cfg.Debug,
		token:     cfg.Token,
	}
}

type profileResponse struct {
	User struct {
		ID            int    `json:"id"`
		CreatedAt     string `json:"createdAt"`
		UpdatedAt     string `json:"updatedAt"`
		Email         string `json:"email"`
		Username      string `json:"username"`
		FirstName     string `json:"firstName"`
		LastName      string `json:"lastName"`
		IsVerified    bool   `json:"isVerified"`
		GPUCredits    int    `json:"gpuCredits"`
		GPUUsageLimit int    `json:"gpuUsageLimit"`
		Organizations []struct {
			ID          int    `json:"id"`
			Username    string `json:"username"`
			FirstName   string `json:"firstName"`
			LastName    string `json:"lastName"`
			Picture     string `json:"picture"`
			TeamName    string `json:"teamName"`
			TeamPicture string `json:"teamPicture"`
		} `json:"organizations"`
	} `json:"user"`
}

func (c *Client) loadTeamID(ctx context.Context) error {
	if c.teamID != 0 {
		return nil
	}
	var resp profileResponse
	if err := c.do(ctx, "GET", "profile", nil, &resp); err != nil {
		return fmt.Errorf("runway: couldn't get profile: %w", err)
	}
	if len(resp.User.Organizations) > 0 {
		c.teamID = resp.User.Organizations[0].ID
		return nil
	}
	c.teamID = resp.User.ID
	return nil
}

type uploadRequest struct {
	Filename      string `json:"filename"`
	NumberOfParts int    `json:"numberOfParts"`
	Type          string `json:"type"`
}

type uploadResponse struct {
	ID           string            `json:"id"`
	UploadURLs   []string          `json:"uploadUrls"`
	UploadHeader map[string]string `json:"uploadHeaders"`
}

type uploadCompleteRequest struct {
	Parts []struct {
		PartNumber int    `json:"PartNumber"`
		ETag       string `json:"ETag"`
	} `json:"parts"`
}

type uploadCompleteResponse struct {
	URL string `json:"url"`
}

type uploadFile struct {
	data      []byte
	extension string
}

func (c *Client) Upload(ctx context.Context, name string, data []byte) (string, error) {
	ext := strings.TrimPrefix(".", filepath.Ext(name))
	file := &uploadFile{
		data:      data,
		extension: ext,
	}

	// Calculate etag
	etag := fmt.Sprintf("%x", md5.Sum(data))

	types := []string{
		"DATASET",
		"DATASET_PREVIEW",
	}
	var imageURL string
	for _, t := range types {
		// Get upload URL
		uploadReq := &uploadRequest{
			Filename:      name,
			NumberOfParts: 1,
			Type:          t,
		}
		var uploadResp uploadResponse
		if err := c.do(ctx, "POST", "uploads", uploadReq, &uploadResp); err != nil {
			return "", fmt.Errorf("runway: couldn't obtain upload url: %w", err)
		}
		if len(uploadResp.UploadURLs) == 0 {
			return "", fmt.Errorf("runway: no upload urls returned")
		}

		// Upload file
		uploadURL := uploadResp.UploadURLs[0]
		if err := c.do(ctx, "PUT", uploadURL, file, nil); err != nil {
			return "", fmt.Errorf("runway: couldn't upload file: %w", err)
		}

		// Complete upload
		completeURL := fmt.Sprintf("uploads/%s/complete", uploadResp.ID)
		completeReq := &uploadCompleteRequest{
			Parts: []struct {
				PartNumber int    `json:"PartNumber"`
				ETag       string `json:"ETag"`
			}{
				{
					PartNumber: 1,
					ETag:       etag,
				},
			},
		}
		var completeResp uploadCompleteResponse
		if err := c.do(ctx, "POST", completeURL, completeReq, &completeResp); err != nil {
			return "", fmt.Errorf("runway: couldn't complete upload: %w", err)
		}
		c.log("runway: upload complete %s", completeResp.URL)
		if completeResp.URL == "" {
			return "", fmt.Errorf("runway: empty image url for type %s", t)
		}
		imageURL = completeResp.URL
	}
	return imageURL, nil
}

type createTaskRequest struct {
	TaskType string `json:"taskType"`
	Internal bool   `json:"internal"`
	Options  struct {
		Seconds        int         `json:"seconds"`
		Gen2Options    gen2Options `json:"gen2Options"`
		Name           string      `json:"name"`
		AssetGroupName string      `json:"assetGroupName"`
		ExploreMode    bool        `json:"exploreMode"`
	} `json:"options"`
	AsTeamID int `json:"asTeamId"`
}

type gen2Options struct {
	Interpolate      bool   `json:"interpolate"`
	Seed             int    `json:"seed"`
	Upscale          bool   `json:"upscale"`
	TextPrompt       string `json:"text_prompt"`
	Watermark        bool   `json:"watermark"`
	ImagePrompt      string `json:"image_prompt,omitempty"`
	InitImage        string `json:"init_image,omitempty"`
	Mode             string `json:"mode"`
	InitVideo        string `json:"init_video,omitempty"`
	MotionScore      int    `json:"motion_score"`
	UseMotionScore   bool   `json:"use_motion_score"`
	UseMotionVectors bool   `json:"use_motion_vectors"`
}

type taskResponse struct {
	Task struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
		TaskType  string `json:"taskType"`
		Options   struct {
			Seconds        int         `json:"seconds"`
			Gen2Options    gen2Options `json:"gen2Options"`
			Name           string      `json:"name"`
			AssetGroupName string      `json:"assetGroupName"`
			ExploreMode    bool        `json:"exploreMode"`
			Recording      bool        `json:"recordingEnabled"`
		} `json:"options"`
		Status                      string      `json:"status"`
		ProgressText                string      `json:"progressText"`
		ProgressRatio               string      `json:"progressRatio"`
		PlaceInLine                 int         `json:"placeInLine"`
		EstimatedTimeToStartSeconds float64     `json:"estimatedTimeToStartSeconds"`
		Artifacts                   []artifact  `json:"artifacts"`
		SharedAsset                 interface{} `json:"sharedAsset"`
	} `json:"task"`
}

type artifact struct {
	ID                 string   `json:"id"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	UserID             int      `json:"userId"`
	CreatedBy          int      `json:"createdBy"`
	TaskID             string   `json:"taskId"`
	ParentAssetGroupId string   `json:"parentAssetGroupId"`
	Filename           string   `json:"filename"`
	URL                string   `json:"url"`
	FileSize           string   `json:"fileSize"`
	IsDirectory        bool     `json:"isDirectory"`
	PreviewURLs        []string `json:"previewUrls"`
	Private            bool     `json:"private"`
	PrivateInTeam      bool     `json:"privateInTeam"`
	Deleted            bool     `json:"deleted"`
	Reported           bool     `json:"reported"`
	Metadata           struct {
		FrameRate  int     `json:"frameRate"`
		Duration   float32 `json:"duration"`
		Dimensions []int   `json:"dimensions"`
	} `json:"metadata"`
}

func (c *Client) Generate(ctx context.Context, assetURL, textPrompt string, interpolate, upscale, watermark, extend bool) (string, error) {
	// Load team ID
	if err := c.loadTeamID(ctx); err != nil {
		return "", fmt.Errorf("runway: couldn't load team id: %w", err)
	}

	// Generate seed
	seed := rand.Intn(1000000000)

	var imageURL string
	var videoURL string
	if extend {
		videoURL = assetURL
	} else {
		imageURL = assetURL
	}

	// Create task
	createReq := &createTaskRequest{
		TaskType: "gen2",
		Internal: false,
		Options: struct {
			Seconds        int         `json:"seconds"`
			Gen2Options    gen2Options `json:"gen2Options"`
			Name           string      `json:"name"`
			AssetGroupName string      `json:"assetGroupName"`
			ExploreMode    bool        `json:"exploreMode"`
		}{
			Seconds: 4,
			Gen2Options: gen2Options{
				Interpolate:    interpolate,
				Seed:           seed,
				Upscale:        upscale,
				TextPrompt:     textPrompt,
				Watermark:      watermark,
				ImagePrompt:    imageURL,
				InitImage:      imageURL,
				InitVideo:      videoURL,
				Mode:           "gen2",
				UseMotionScore: true,
				MotionScore:    22,
			},
			Name:           fmt.Sprintf("Gen-2, %d", seed),
			AssetGroupName: "Gen-2",
			ExploreMode:    false,
		},
		AsTeamID: c.teamID,
	}
	var taskResp taskResponse
	if err := c.do(ctx, "POST", "tasks", createReq, &taskResp); err != nil {
		return "", fmt.Errorf("runway: couldn't create task: %w", err)
	}

	// Wait for task to finish
	for {
		switch taskResp.Task.Status {
		case "SUCCEEDED":
			if len(taskResp.Task.Artifacts) == 0 {
				return "", fmt.Errorf("runway: no artifacts returned")
			}
			if taskResp.Task.Artifacts[0].URL == "" {
				return "", fmt.Errorf("runway: empty artifact url")
			}
			return taskResp.Task.Artifacts[0].URL, nil
		case "PENDING", "RUNNING":
			c.log("runway: task %s: %s", taskResp.Task.ID, taskResp.Task.ProgressRatio)
		default:
			return "", fmt.Errorf("runway: task failed: %s", taskResp.Task.Status)
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("runway: %w", ctx.Err())
		case <-time.After(5 * time.Second):
		}

		path := fmt.Sprintf("tasks/%s?asTeamId=%d", taskResp.Task.ID, c.teamID)
		if err := c.do(ctx, "GET", path, nil, &taskResp); err != nil {
			return "", fmt.Errorf("runway: couldn't get task: %w", err)
		}
	}
}

type assetDeleteRequest struct {
}

type assetDeleteResponse struct {
	Success bool `json:"success"`
}

// TODO: Delete asset by url instead
func (c *Client) DeleteAsset(ctx context.Context, id string) error {
	path := fmt.Sprintf("assets/%s", id)
	var resp assetDeleteResponse
	if err := c.do(ctx, "DELETE", path, &assetDeleteRequest{}, &resp); err != nil {
		return fmt.Errorf("runway: couldn't delete asset %s: %w", id, err)
	}
	if !resp.Success {
		return fmt.Errorf("runway: couldn't delete asset %s", id)
	}
	return nil
}

func (c *Client) log(format string, args ...interface{}) {
	if c.debug {
		format += "\n"
		log.Printf(format, args...)
	}
}

var backoff = []time.Duration{
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	maxAttempts := 3
	attempts := 0
	var err error
	for {
		if err != nil {
			log.Println("retrying...", err)
		}
		err = c.doAttempt(ctx, method, path, in, out)
		if err == nil {
			return nil
		}
		// Increase attempts and check if we should stop
		attempts++
		if attempts >= maxAttempts {
			return err
		}
		// If the error is temporary retry
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			continue
		}
		// Check status code
		var errStatus errStatusCode
		if errors.As(err, &errStatus) {
			switch int(errStatus) {
			// These errors are retriable but we should wait before retry
			case http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusTooManyRequests:
			default:
				return err
			}

			idx := attempts - 1
			if idx >= len(backoff) {
				idx = len(backoff) - 1
			}
			wait := backoff[idx]
			c.log("server seems to be down, waiting %s before retrying\n", wait)
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
			}
			continue
		}
		return err
	}
}

type errStatusCode int

func (e errStatusCode) Error() string {
	return fmt.Sprintf("%d", e)
}

func (c *Client) doAttempt(ctx context.Context, method, path string, in, out any) error {
	var body []byte
	var reqBody io.Reader
	contentType := "application/json"
	if f, ok := in.(*uploadFile); ok {
		body = f.data
		ext := f.extension
		if ext == "jpg" {
			ext = "jpeg"
		}
		contentType = fmt.Sprintf("image/%s", ext)
		reqBody = bytes.NewReader(body)
	} else if in != nil {
		var err error
		body, err = json.Marshal(in)
		if err != nil {
			return fmt.Errorf("runway: couldn't marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(body)
	}
	logBody := string(body)
	if len(logBody) > 100 {
		logBody = logBody[:100] + "..."
	}
	c.log("runway: do %s %s %s", method, path, logBody)

	// Check if path is absolute
	u := fmt.Sprintf("https://api.runwayml.com/v1/%s", path)
	var uploadLen int
	if strings.HasPrefix(path, "http") {
		u = path
		uploadLen = len(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return fmt.Errorf("runway: couldn't create request: %w", err)
	}
	c.addHeaders(req, contentType, uploadLen)

	unlock := c.ratelimit.Lock(ctx)
	defer unlock()

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("runway: couldn't %s %s: %w", method, u, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("runway: couldn't read response body: %w", err)
	}
	c.log("runway: response %s %s %d %s", method, path, resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK {
		errMessage := string(respBody)
		if len(errMessage) > 100 {
			errMessage = errMessage[:100] + "..."
		}
		_ = os.WriteFile(fmt.Sprintf("logs/debug_%s.json", time.Now().Format("20060102_150405")), respBody, 0644)
		return fmt.Errorf("runway: %s %s returned (%s): %w", method, u, errMessage, errStatusCode(resp.StatusCode))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			// Write response body to file for debugging.
			_ = os.WriteFile(fmt.Sprintf("logs/debug_%s.json", time.Now().Format("20060102_150405")), respBody, 0644)
			return fmt.Errorf("runway: couldn't unmarshal response body (%T): %w", out, err)
		}
	}
	return nil
}

func (c *Client) addHeaders(req *http.Request, contentType string, uploadLen int) {
	if uploadLen > 0 {
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Content-Length", fmt.Sprintf("%d", uploadLen))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
	} else {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
		req.Header.Set("Sec-Fetch-Site", "same-site")
	}
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", "https://app.runwayml.com")
	req.Header.Set("Referer", "https://app.runwayml.com/")
	req.Header.Set("Sec-Ch-Ua", `"Not.A/Brand";v="8", "Chromium";v="114", "Microsoft Edge";v="114"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"Windows\"")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	// TODO: Add sentry trace if needed.
	// req.Header.Set("Sentry-Trace", "TODO")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36 Edg/114.0.1823.82")
}
