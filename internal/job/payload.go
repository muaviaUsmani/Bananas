package job

import (
	"encoding/json"
	"fmt"

	"github.com/muaviaUsmani/bananas/internal/serialization"
	"google.golang.org/protobuf/proto"
)

var (
	// DefaultSerializer is the global serializer instance
	// Set to protobuf by default for better performance
	DefaultSerializer = serialization.NewProtobufSerializer()
)

// NewJobWithProto creates a new job with a protobuf payload
// The payload is automatically serialized to protobuf format
func NewJobWithProto(name string, payload proto.Message, priority JobPriority, description ...string) (*Job, error) {
	data, err := DefaultSerializer.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize protobuf payload: %w", err)
	}

	return NewJob(name, data, priority, description...), nil
}

// NewJobWithJSON creates a new job with a JSON payload (legacy compatibility)
// The payload is automatically serialized to JSON format
func NewJobWithJSON(name string, payload interface{}, priority JobPriority, description ...string) (*Job, error) {
	jsonSerializer := serialization.NewJSONSerializer()
	data, err := jsonSerializer.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize JSON payload: %w", err)
	}

	return NewJob(name, data, priority, description...), nil
}

// GetPayloadFormat returns the format of the job's payload
func (j *Job) GetPayloadFormat() (serialization.PayloadFormat, error) {
	return DefaultSerializer.GetFormat(j.Payload)
}

// IsProtobufPayload returns true if the job's payload is in protobuf format
func (j *Job) IsProtobufPayload() bool {
	return DefaultSerializer.IsProtobuf(j.Payload)
}

// IsJSONPayload returns true if the job's payload is in JSON format
func (j *Job) IsJSONPayload() bool {
	return DefaultSerializer.IsJSON(j.Payload)
}

// UnmarshalPayload deserializes the job's payload into the provided type
// The format is automatically detected (JSON or protobuf)
func (j *Job) UnmarshalPayload(v interface{}) error {
	return DefaultSerializer.Unmarshal(j.Payload, v)
}

// UnmarshalPayloadProto deserializes the job's payload into a protobuf message
func (j *Job) UnmarshalPayloadProto(msg proto.Message) error {
	return DefaultSerializer.Unmarshal(j.Payload, msg)
}

// UnmarshalPayloadJSON deserializes the job's payload into a Go value (legacy)
func (j *Job) UnmarshalPayloadJSON(v interface{}) error {
	format, payload, err := DefaultSerializer.DetectFormat(j.Payload)
	if err != nil {
		return err
	}

	if format != serialization.FormatJSON {
		return fmt.Errorf("payload is not in JSON format")
	}

	return json.Unmarshal(payload, v)
}

// SetPayload sets the job's payload with automatic serialization
// Detects if the value is a proto.Message and serializes accordingly
func (j *Job) SetPayload(v interface{}) error {
	var data []byte
	var err error

	// Check if it's a protobuf message
	if msg, ok := v.(proto.Message); ok {
		data, err = DefaultSerializer.Marshal(msg)
	} else {
		// Fallback to JSON for other types
		jsonSerializer := serialization.NewJSONSerializer()
		data, err = jsonSerializer.Marshal(v)
	}

	if err != nil {
		return err
	}

	j.Payload = data
	return nil
}
