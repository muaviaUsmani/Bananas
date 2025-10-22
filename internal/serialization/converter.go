package serialization

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/muaviaUsmani/bananas/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// JSONToProtoPackageIngestion converts a JSON payload to PackageIngestionTask
func JSONToProtoPackageIngestion(jsonData map[string]interface{}) (*supplychain.PackageIngestionTask, error) {
	task := &supplychain.PackageIngestionTask{}

	if v, ok := jsonData["package_name"].(string); ok {
		task.PackageName = v
	}
	if v, ok := jsonData["version"].(string); ok {
		task.Version = v
	}
	if v, ok := jsonData["registry"].(string); ok {
		task.Registry = v
	}
	if v, ok := jsonData["download_stats"].(float64); ok {
		task.DownloadStats = int64(v)
	}

	// Handle array fields
	if v, ok := jsonData["maintainers"].([]interface{}); ok {
		maintainers := make([]string, len(v))
		for i, m := range v {
			if s, ok := m.(string); ok {
				maintainers[i] = s
			}
		}
		task.Maintainers = maintainers
	}

	if v, ok := jsonData["licenses"].([]interface{}); ok {
		licenses := make([]string, len(v))
		for i, l := range v {
			if s, ok := l.(string); ok {
				licenses[i] = s
			}
		}
		task.Licenses = licenses
	}

	// Handle timestamp
	if v, ok := jsonData["publish_timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			task.PublishTimestamp = timestamppb.New(t)
		}
	}

	// Additional fields
	if v, ok := jsonData["homepage_url"].(string); ok {
		task.HomepageUrl = v
	}
	if v, ok := jsonData["repository_url"].(string); ok {
		task.RepositoryUrl = v
	}
	if v, ok := jsonData["description"].(string); ok {
		task.Description = v
	}

	// Metadata map
	if v, ok := jsonData["metadata"].(map[string]interface{}); ok {
		task.Metadata = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				task.Metadata[k] = s
			}
		}
	}

	return task, nil
}

// ProtoToJSONPackageIngestion converts PackageIngestionTask to JSON-compatible map
func ProtoToJSONPackageIngestion(task *supplychain.PackageIngestionTask) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	result["package_name"] = task.PackageName
	result["version"] = task.Version
	result["registry"] = task.Registry
	result["download_stats"] = task.DownloadStats
	result["maintainers"] = task.Maintainers
	result["licenses"] = task.Licenses

	if task.PublishTimestamp != nil {
		result["publish_timestamp"] = task.PublishTimestamp.AsTime().Format(time.RFC3339)
	}

	result["homepage_url"] = task.HomepageUrl
	result["repository_url"] = task.RepositoryUrl
	result["description"] = task.Description
	result["metadata"] = task.Metadata

	return result, nil
}

// ToProtoMessage converts a generic payload to a protobuf message based on task type
func ToProtoMessage(taskType string, payload []byte) (interface{}, error) {
	// Try to parse as JSON first
	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON payload: %w", err)
	}

	switch taskType {
	case "package_ingestion":
		return JSONToProtoPackageIngestion(jsonData)
	case "dependency_resolution":
		// TODO: Implement other converters as needed
		return nil, fmt.Errorf("converter not implemented for task type: %s", taskType)
	case "vulnerability_scan":
		return nil, fmt.Errorf("converter not implemented for task type: %s", taskType)
	case "health_metrics":
		return nil, fmt.Errorf("converter not implemented for task type: %s", taskType)
	default:
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}
}

// FromProtoMessage converts a protobuf message to a JSON payload
func FromProtoMessage(msg interface{}) ([]byte, error) {
	switch v := msg.(type) {
	case *supplychain.PackageIngestionTask:
		jsonMap, err := ProtoToJSONPackageIngestion(v)
		if err != nil {
			return nil, err
		}
		return json.Marshal(jsonMap)
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
}
