package runway

import (
	"context"
	"crypto/md5"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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
		if _, err := c.do(ctx, "POST", "uploads", uploadReq, &uploadResp); err != nil {
			return "", fmt.Errorf("runway: couldn't obtain upload url: %w", err)
		}
		if len(uploadResp.UploadURLs) == 0 {
			return "", fmt.Errorf("runway: no upload urls returned")
		}

		// Upload file
		uploadURL := uploadResp.UploadURLs[0]
		if _, err := c.do(ctx, "PUT", uploadURL, file, nil); err != nil {
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
		if _, err := c.do(ctx, "POST", completeURL, completeReq, &completeResp); err != nil {
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
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	AssetGroupName string `json:"assetGroupName"`
}

type taskResponse struct {
	Task struct {
		ID                          string      `json:"id"`
		Name                        string      `json:"name"`
		CreatedAt                   string      `json:"createdAt"`
		UpdatedAt                   string      `json:"updatedAt"`
		TaskType                    string      `json:"taskType"`
		Options                     any         `json:"options"`
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
	PreviewURLs []string `json:"previewUrls"`
}

type GenerateRequest struct {
	Model       string
	AssetURL    string
	Prompt      string
	Interpolate bool
	Upscale     bool
	Watermark   bool
	Extend      bool
	Width       int
	Height      int
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

	width := cfg.Width
	height := cfg.Height
	if width == 0 || height == 0 {
		width = 1280
		height = 768
	}

	// Create task
	var createReq any
	switch cfg.Model {
	case "gen2":
		name := fmt.Sprintf("Gen-2 %d, %s", seed, cfg.Prompt)
		if len(name) > 44 {
			name = name[:44]
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
				AssetGroupName: "Generative Video",
				ExploreMode:    false,
			},
			AsTeamID: c.teamID,
		}
	case "gen3":
		name := fmt.Sprintf("Gen-3 Alpha %d, %s", seed, cfg.Prompt)
		if len(name) > 44 {
			name = name[:44]
		}
		createReq = &createGen3TaskRequest{
			TaskType: "europa",
			Internal: false,
			Options: gen3Options{
				Name:           name,
				Seconds:        10,
				TextPrompt:     cfg.Prompt,
				Seed:           seed,
				ExploreMode:    false,
				Watermark:      cfg.Watermark,
				EnhancePrompt:  true,
				Width:          width,
				Height:         height,
				AssetGroupName: "Generative Video",
			},
			AsTeamID: c.teamID,
		}
	default:
		return nil, fmt.Errorf("runway: unknown model %s", cfg.Model)
	}
	var taskResp taskResponse
	if _, err := c.do(ctx, "POST", "tasks", createReq, &taskResp); err != nil {
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
			return &Generation{
				ID:          artifact.ID,
				URL:         artifact.URL,
				PreviewURLs: artifact.PreviewURLs,
			}, nil
		case "PENDING", "RUNNING":
			c.log("runway: task %s: %s", taskResp.Task.ID, taskResp.Task.ProgressRatio)
		default:
			return nil, fmt.Errorf("runway: task failed: %s", taskResp.Task.Status)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("runway: %w", ctx.Err())
		case <-time.After(5 * time.Second):
		}

		path := fmt.Sprintf("tasks/%s?asTeamId=%d", taskResp.Task.ID, c.teamID)
		if _, err := c.do(ctx, "GET", path, nil, &taskResp); err != nil {
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

func (c *Client) GetAsset(ctx context.Context, id string) (string, error) {
	path := fmt.Sprintf("assets/%s", id)
	var resp assetResponse
	if _, err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return "", fmt.Errorf("runway: couldn't get asset %s: %w", id, err)
	}
	if resp.Asset.URL == "" {
		return "", fmt.Errorf("runway: empty asset url")
	}
	return resp.Asset.URL, nil
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
