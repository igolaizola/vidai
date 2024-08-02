package runway

import (
	"context"
	"crypto/md5"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

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
	if _, err := c.do(ctx, "GET", "profile", nil, &resp); err != nil {
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

type datasetRequest struct {
	FileCount        int      `json:"fileCount"`
	Name             string   `json:"name"`
	UploadID         string   `json:"uploadId"`
	PreviewUploadIDs []string `json:"previewUploadIds"`
	Type             struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		IsDirectory bool   `json:"isDirectory"`
	} `json:"type"`
	AsTeamID int `json:"asTeamId"`
}

type datasetResponse struct {
	Dataset struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
		// More fields that are not used
	} `json:"dataset"`
}

func (c *Client) Upload(ctx context.Context, name string, data []byte) (string, string, error) {
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
	var uploadID, previewUploadID string
	for _, t := range types {
		// Get upload URL
		uploadReq := &uploadRequest{
			Filename:      name,
			NumberOfParts: 1,
			Type:          t,
		}
		var uploadResp uploadResponse
		if _, err := c.do(ctx, "POST", "uploads", uploadReq, &uploadResp); err != nil {
			return "", "", fmt.Errorf("runway: couldn't obtain upload url: %w", err)
		}
		if len(uploadResp.UploadURLs) == 0 {
			return "", "", fmt.Errorf("runway: no upload urls returned")
		}

		// Upload file
		uploadURL := uploadResp.UploadURLs[0]
		if _, err := c.do(ctx, "PUT", uploadURL, file, nil); err != nil {
			return "", "", fmt.Errorf("runway: couldn't upload file: %w", err)
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
		if _, err := c.do(ctx, "POST", completeURL, completeReq, &completeResp); err != nil {
			return "", "", fmt.Errorf("runway: couldn't complete upload: %w", err)
		}

		c.log("runway: upload complete %s", completeResp.URL)
		if completeResp.URL == "" {
			return "", "", fmt.Errorf("runway: empty image url for type %s", t)
		}
		imageURL = completeResp.URL

		switch t {
		case "DATASET":
			uploadID = uploadResp.ID
		case "DATASET_PREVIEW":
			previewUploadID = uploadResp.ID
		}
	}

	// Dataset request
	datasetReq := &datasetRequest{
		FileCount:        1,
		Name:             name,
		PreviewUploadIDs: []string{previewUploadID},
		UploadID:         uploadID,
		Type: struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			IsDirectory bool   `json:"isDirectory"`
		}{
			Name:        "image",
			Type:        "image",
			IsDirectory: false,
		},
		AsTeamID: c.teamID,
	}
	var datasetResp datasetResponse
	if _, err := c.do(ctx, "POST", "datasets", datasetReq, &datasetResp); err != nil {
		return "", "", fmt.Errorf("runway: couldn't create dataset: %w", err)
	}
	if datasetResp.Dataset.URL == "" || datasetResp.Dataset.ID == "" {
		return "", "", fmt.Errorf("runway: empty dataset url or id")
	}

	return imageURL, datasetResp.Dataset.ID, nil
}

type deleteRequest struct {
	AsTeamID int `json:"asTeamId"`
}

type deleteResponse struct {
	Success bool `json:"success"`
}

func (c *Client) Delete(ctx context.Context, assetID string) error {
	path := fmt.Sprintf("assets/%s", assetID)
	req := &deleteRequest{
		AsTeamID: c.teamID,
	}
	var resp deleteResponse
	b, err := c.do(ctx, "DELETE", path, req, &resp)
	if err != nil {
		return fmt.Errorf("runway: couldn't delete asset %s: %w", assetID, err)
	}
	if !resp.Success {
		return fmt.Errorf("runway: couldn't delete asset %s: %s", assetID, string(b))
	}
	return nil
}

type createGen2TaskRequest struct {
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
	Width            int    `json:"width"`
	Height           int    `json:"height"`
}

type createGen3TaskRequest struct {
	TaskType string      `json:"taskType"`
	Internal bool        `json:"internal"`
	Options  gen3Options `json:"options"`
	AsTeamID int         `json:"asTeamId"`
}

type gen3Options struct {
	Name           string `json:"name"`
	Seconds        int    `json:"seconds"`
	TextPrompt     string `json:"text_prompt"`
	Seed           int    `json:"seed"`
	ExploreMode    bool   `json:"exploreMode"`
	Watermark      bool   `json:"watermark"`
	EnhancePrompt  bool   `json:"enhance_prompt"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	InitImage      string `json:"init_image,omitempty"`
	AssetGroupName string `json:"assetGroupName"`
}

type taskResponse struct {
	Task taskData `json:"task"`
}

type taskData struct {
	ID                          string      `json:"id"`
	Name                        string      `json:"name"`
	CreatedAt                   string      `json:"createdAt"`
	UpdatedAt                   string      `json:"updatedAt"`
	TaskType                    string      `json:"taskType"`
	Options                     any         `json:"options"`
	Status                      string      `json:"status"`
	Error                       taskError   `json:"error"`
	ProgressText                string      `json:"progressText"`
	ProgressRatio               string      `json:"progressRatio"`
	PlaceInLine                 int         `json:"placeInLine"`
	EstimatedTimeToStartSeconds float64     `json:"estimatedTimeToStartSeconds"`
	Artifacts                   []artifact  `json:"artifacts"`
	SharedAsset                 interface{} `json:"sharedAsset"`
}

type taskError struct {
	ErrorMessage       string `json:"errorMessage"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	ModerationCategory string `json:"moderation_category"`
	TallyAsimov        bool   `json:"tally_asimov"`
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
	FileSize           any      `json:"fileSize"`
	FileExtension      string   `json:"fileExtStandardized"`
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
		Size       struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"size"`
	} `json:"metadata"`
}

type Generation struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	S3URL       string   `json:"s3Url"`
	PreviewURLs []string `json:"previewUrls"`
}

type GenerateRequest struct {
	Model       string
	AssetURL    string
	AssetName   string
	Prompt      string
	Interpolate bool
	Upscale     bool
	Watermark   bool
	Extend      bool
	Width       int
	Height      int
	ExploreMode bool
}

type Error struct {
	data taskData
	raw  []byte
}

func (e *Error) Error() string {
	return fmt.Sprintf("runway: task %s %q (%q, %q)", e.data.Status, e.data.Error.Message, e.data.Error.Reason, e.data.Error.ModerationCategory)
}

func (e *Error) Debug() string {
	return string(e.raw)
}

func (e *Error) Reason() string {
	return e.data.Error.Reason
}

func (e *Error) Temporary() bool {
	r := e.data.Error.Reason
	switch {
	case r == "SAFETY.INPUT.TEXT":
		return false
	case strings.HasPrefix(r, "INTERNAL.BAD_OUTPUT."):
		return true
	case r == "":
		return true
	default:
		return false
	}
}

func (c *Client) Generate(ctx context.Context, cfg *GenerateRequest) (*Generation, error) {
	// Load team ID
	if err := c.loadTeamID(ctx); err != nil {
		return nil, fmt.Errorf("runway: couldn't load team id: %w", err)
	}

	// Generate seed between 2000000000 and 2999999999
	seed := rand.Intn(1000000000) + 2000000000

	var imageURL string
	var videoURL string
	if cfg.Extend {
		videoURL = cfg.AssetURL
	} else {
		imageURL = cfg.AssetURL
	}

	var width, height int
	var resolution string
	if len(imageURL) == 0 {
		width = cfg.Width
		height = cfg.Height
		if width == 0 || height == 0 {
			width = 1280
			height = 768
		}
	} else {
		resolution = "720p"
	}

	// Create task
	var createReq any
	switch cfg.Model {
	case "gen2":
		name := fmt.Sprintf("Gen-2 %d, %s", seed, cfg.Prompt)
		if len(cfg.Prompt) > 0 {
			v := cfg.Prompt
			if len(v) > 20 {
				v = v[:20]
			}
			name = fmt.Sprintf("%s, %s", name, v)
		}
		if len(cfg.AssetName) > 0 {
			v := cfg.AssetName
			if len(v) > 20 {
				v = v[:20]
			}
			name = fmt.Sprintf("%s, %s", name, v)
		}

		createReq = &createGen2TaskRequest{
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
					Interpolate:    cfg.Interpolate,
					Seed:           seed,
					Upscale:        cfg.Upscale,
					TextPrompt:     cfg.Prompt,
					Watermark:      cfg.Watermark,
					ImagePrompt:    imageURL,
					InitImage:      imageURL,
					InitVideo:      videoURL,
					Mode:           "gen2",
					UseMotionScore: true,
					MotionScore:    22,
					Width:          width,
					Height:         height,
				},
				Name:           name,
				AssetGroupName: c.folder,
				ExploreMode:    cfg.ExploreMode,
			},
			AsTeamID: c.teamID,
		}
	case "gen3":
		name := fmt.Sprintf("Gen-3 Alpha %d", seed)
		if len(cfg.Prompt) > 0 {
			v := cfg.Prompt
			if len(v) > 20 {
				v = v[:20]
			}
			name = fmt.Sprintf("%s, %s", name, v)
		}
		if len(cfg.AssetName) > 0 {
			v := cfg.AssetName
			if len(v) > 20 {
				v = v[:20]
			}
			name = fmt.Sprintf("%s, %s", name, v)
		}
		createReq = &createGen3TaskRequest{
			TaskType: "gen3a",
			Internal: false,
			Options: gen3Options{
				Name:           name,
				Seconds:        10,
				TextPrompt:     cfg.Prompt,
				Seed:           seed,
				ExploreMode:    cfg.ExploreMode,
				Watermark:      cfg.Watermark,
				EnhancePrompt:  true,
				Width:          width,
				Height:         height,
				InitImage:      imageURL,
				Resolution:     resolution,
				AssetGroupName: c.folder,
			},
			AsTeamID: c.teamID,
		}
	default:
		return nil, fmt.Errorf("runway: unknown model %s", cfg.Model)
	}
	var taskResp taskResponse
	b, err := c.do(ctx, "POST", "tasks", createReq, &taskResp)
	if err != nil {
		return nil, fmt.Errorf("runway: couldn't create task: %w", err)
	}

	// Wait for task to finish
	for {
		switch taskResp.Task.Status {
		case "SUCCEEDED":
			if len(taskResp.Task.Artifacts) == 0 {
				return nil, fmt.Errorf("runway: no artifacts returned")
			}
			artifact := taskResp.Task.Artifacts[0]
			if artifact.URL == "" {
				return nil, fmt.Errorf("runway: empty artifact url")
			}
			s3URL, err := ToS3URL(artifact.URL)
			if err != nil {
				return nil, err
			}
			return &Generation{
				ID:          artifact.ID,
				URL:         artifact.URL,
				S3URL:       s3URL,
				PreviewURLs: artifact.PreviewURLs,
			}, nil
		case "PENDING", "RUNNING", "THROTTLED":
			c.log("runway: task %s: %s", taskResp.Task.ID, taskResp.Task.ProgressRatio)
		default:
			return nil, &Error{data: taskResp.Task, raw: b}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("runway: %w", ctx.Err())
		case <-time.After(5 * time.Second):
		}

		path := fmt.Sprintf("tasks/%s?asTeamId=%d", taskResp.Task.ID, c.teamID)
		b, err = c.do(ctx, "GET", path, nil, &taskResp)
		if err != nil {
			return nil, fmt.Errorf("runway: couldn't get task: %w", err)
		}
	}
}

type assetDeleteRequest struct {
}

type assetDeleteResponse struct {
	Success bool `json:"success"`
}

type assetResponse struct {
	Asset artifact `json:"asset"`
}

func (c *Client) DeleteAsset(ctx context.Context, id string) error {
	path := fmt.Sprintf("assets/%s", id)
	var resp assetDeleteResponse
	if _, err := c.do(ctx, "DELETE", path, &assetDeleteRequest{}, &resp); err != nil {
		return fmt.Errorf("runway: couldn't delete asset %s: %w", id, err)
	}
	if !resp.Success {
		return fmt.Errorf("runway: couldn't delete asset %s", id)
	}
	return nil
}

func (c *Client) GetAsset(ctx context.Context, id string) (string, string, []string, error) {
	path := fmt.Sprintf("assets/%s", id)
	var resp assetResponse
	if _, err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return "", "", nil, fmt.Errorf("runway: couldn't get asset %s: %w", id, err)
	}
	if resp.Asset.URL == "" {
		return "", "", nil, fmt.Errorf("runway: empty asset url")
	}

	// Find the UUID in the URL
	s3URL, err := ToS3URL(resp.Asset.URL)
	if err != nil {
		return "", "", nil, fmt.Errorf("runway: couldn't convert asset URL to AWS URL: %w", err)
	}
	return s3URL, resp.Asset.URL, resp.Asset.PreviewURLs, nil
}

func (c *Client) Download(ctx context.Context, u, output string) error {
	b, err := c.do(ctx, "GET", u, nil, nil)
	if err != nil {
		return fmt.Errorf("runway: couldn't download video: %w", err)
	}
	// Write video to output
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("runway: couldn't create output directory: %w", err)
	}
	if err := os.WriteFile(output, b, 0644); err != nil {
		return fmt.Errorf("runway: couldn't write video to file: %w", err)
	}
	return nil
}

var uuidRegex = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)

func ToS3URL(u string) (string, error) {
	// Find the UUID in the URL
	uuid := uuidRegex.FindString(u)
	if uuid == "" {
		return "", fmt.Errorf("runway: couldn't find UUID in asset URL")
	}
	return fmt.Sprintf("https://runway-task-artifacts.s3.amazonaws.com/%s.mp4", uuid), nil
}
