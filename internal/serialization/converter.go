package serialization

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/muaviaUsmani/bananas/proto/gen"
)

// JSONToProtoEmail converts a JSON payload to EmailTask
func JSONToProtoEmail(jsonData map[string]interface{}) (*tasks.EmailTask, error) {
	task := &tasks.EmailTask{}

	if v, ok := jsonData["to"].(string); ok {
		task.To = v
	}
	if v, ok := jsonData["from"].(string); ok {
		task.From = v
	}
	if v, ok := jsonData["subject"].(string); ok {
		task.Subject = v
	}
	if v, ok := jsonData["body_text"].(string); ok {
		task.BodyText = v
	}
	if v, ok := jsonData["body_html"].(string); ok {
		task.BodyHtml = v
	}

	// Handle array fields
	if v, ok := jsonData["cc"].([]interface{}); ok {
		cc := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				cc[i] = s
			}
		}
		task.Cc = cc
	}

	if v, ok := jsonData["bcc"].([]interface{}); ok {
		bcc := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				bcc[i] = s
			}
		}
		task.Bcc = bcc
	}

	// Handle headers map
	if v, ok := jsonData["headers"].(map[string]interface{}); ok {
		task.Headers = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				task.Headers[k] = s
			}
		}
	}

	// Handle timestamp
	if v, ok := jsonData["scheduled_for"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			task.ScheduledFor = timestamppb.New(t)
		}
	}

	return task, nil
}

// ProtoToJSONEmail converts EmailTask to JSON-compatible map
func ProtoToJSONEmail(task *tasks.EmailTask) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	result["to"] = task.To
	result["from"] = task.From
	result["subject"] = task.Subject
	result["body_text"] = task.BodyText
	result["body_html"] = task.BodyHtml
	result["cc"] = task.Cc
	result["bcc"] = task.Bcc
	result["headers"] = task.Headers

	if task.ScheduledFor != nil {
		result["scheduled_for"] = task.ScheduledFor.AsTime().Format(time.RFC3339)
	}

	return result, nil
}

// JSONToProtoGeneric converts a JSON payload to GenericTask
func JSONToProtoGeneric(jsonData map[string]interface{}) (*tasks.GenericTask, error) {
	task := &tasks.GenericTask{}

	if v, ok := jsonData["task_id"].(string); ok {
		task.TaskId = v
	}
	if v, ok := jsonData["task_type"].(string); ok {
		task.TaskType = v
	}
	if v, ok := jsonData["priority"].(float64); ok {
		task.Priority = int32(v)
	}

	// Handle data map
	if v, ok := jsonData["data"].(map[string]interface{}); ok {
		task.Data = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				task.Data[k] = s
			}
		}
	}

	// Handle tags array
	if v, ok := jsonData["tags"].([]interface{}); ok {
		tags := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				tags[i] = s
			}
		}
		task.Tags = tags
	}

	// Handle timestamp
	if v, ok := jsonData["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			task.CreatedAt = timestamppb.New(t)
		}
	}

	return task, nil
}

// ProtoToJSONGeneric converts GenericTask to JSON-compatible map
func ProtoToJSONGeneric(task *tasks.GenericTask) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	result["task_id"] = task.TaskId
	result["task_type"] = task.TaskType
	result["priority"] = task.Priority
	result["data"] = task.Data
	result["tags"] = task.Tags

	if task.CreatedAt != nil {
		result["created_at"] = task.CreatedAt.AsTime().Format(time.RFC3339)
	}

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
	case "email":
		return JSONToProtoEmail(jsonData)
	case "generic":
		return JSONToProtoGeneric(jsonData)
	default:
		// For unknown types, use GenericTask
		return JSONToProtoGeneric(jsonData)
	}
}

// FromProtoMessage converts a protobuf message to a JSON payload
func FromProtoMessage(msg interface{}) ([]byte, error) {
	switch v := msg.(type) {
	case *tasks.EmailTask:
		jsonMap, err := ProtoToJSONEmail(v)
		if err != nil {
			return nil, err
		}
		return json.Marshal(jsonMap)
	case *tasks.GenericTask:
		jsonMap, err := ProtoToJSONGeneric(v)
		if err != nil {
			return nil, err
		}
		return json.Marshal(jsonMap)
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
}
