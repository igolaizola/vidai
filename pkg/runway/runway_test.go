package runway

import (
	"encoding/json"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	js := `{
	"task": {
		"id": "00000000-0000-0000-0000-000000000000",
		"name": "Gen-2, 100000",
		"image": null,
		"createdAt": "2024-01-01T01:01:01.001Z",
		"updatedAt": "2024-01-01T01:01:01.001Z",
		"taskType": "gen2",
		"options": {
			"seconds": 4,
			"gen2Options": {
				"interpolate": true,
				"seed": 100000,
				"upscale": true,
				"text_prompt": "",
				"watermark": false,
				"image_prompt": "https://a.url.test",
				"init_image": "https://a.url.test",
				"mode": "gen2",
				"motion_score": 22,
				"use_motion_score": true,
				"use_motion_vectors": false
			},
			"name": "Gen-2, 100000",
			"assetGroupName": "Gen-2",
			"exploreMode": false,
			"recordingEnabled": true
		},
		"status": "SUCCEEDED",
		"error": null,
		"progressText": null,
		"progressRatio": "1",
		"placeInLine": null,
		"estimatedTimeToStartSeconds": null,
		"artifacts": [
			{
				"id": "00000000-0000-0000-0000-000000000000",
				"createdAt": "2024-01-01T01:01:01.001Z",
				"updatedAt": "2024-01-01T01:01:01.001Z",
				"userId": 100000,
				"createdBy": 100000,
				"taskId": "00000000-0000-0000-0000-000000000000",
				"parentAssetGroupId": "00000000-0000-0000-0000-000000000000",
				"filename": "Gen-2, 100000.mp4",
				"url": "https://a.url.test",
				"fileSize": "100000",
				"isDirectory": false,
				"previewUrls": [
					"https://a.url.test",
					"https://a.url.test",
					"https://a.url.test",
					"https://a.url.test"
				],
				"private": true,
				"privateInTeam": true,
				"deleted": false,
				"reported": false,
				"metadata": {
					"frameRate": 24,
					"duration": 8.1,
					"dimensions": [
						2816,
						1536
					],
					"size": {
						"width": 2816,
						"height": 1536
					}
				}
			}
		],
		"sharedAsset": null
	}
}`
	var resp taskResponse
	if err := json.Unmarshal([]byte(js), &resp); err != nil {
		t.Fatal(err)
	}
}
