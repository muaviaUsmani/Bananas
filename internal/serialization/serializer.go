package serialization

import (
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// PayloadFormat represents the serialization format used for a payload
type PayloadFormat byte

const (
	// FormatJSON represents JSON serialization (legacy/default for backward compatibility)
	FormatJSON PayloadFormat = 0x00

	// FormatProtobuf represents Protocol Buffers serialization
	FormatProtobuf PayloadFormat = 0x01
)

var (
	// ErrUnknownFormat is returned when the payload format cannot be determined
	ErrUnknownFormat = errors.New("unknown payload format")

	// ErrMarshalFailed is returned when marshaling fails
	ErrMarshalFailed = errors.New("failed to marshal payload")

	// ErrUnmarshalFailed is returned when unmarshaling fails
	ErrUnmarshalFailed = errors.New("failed to unmarshal payload")
)

// Serializer handles payload serialization with format detection
type Serializer struct {
	// DefaultFormat is the format to use when serializing new payloads
	DefaultFormat PayloadFormat
}

// NewSerializer creates a new serializer with the specified default format
func NewSerializer(defaultFormat PayloadFormat) *Serializer {
	return &Serializer{
		DefaultFormat: defaultFormat,
	}
}

// NewProtobufSerializer creates a serializer that defaults to protobuf format
func NewProtobufSerializer() *Serializer {
	return &Serializer{
		DefaultFormat: FormatProtobuf,
	}
}

// NewJSONSerializer creates a serializer that defaults to JSON format (legacy)
func NewJSONSerializer() *Serializer {
	return &Serializer{
		DefaultFormat: FormatJSON,
	}
}

// Marshal serializes a payload using the configured default format
// Returns the serialized bytes with a format prefix
func (s *Serializer) Marshal(v interface{}) ([]byte, error) {
	return s.MarshalWithFormat(v, s.DefaultFormat)
}

// MarshalWithFormat serializes a payload using the specified format
func (s *Serializer) MarshalWithFormat(v interface{}, format PayloadFormat) ([]byte, error) {
	var data []byte
	var err error

	switch format {
	case FormatJSON:
		data, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w (JSON): %v", ErrMarshalFailed, err)
		}

	case FormatProtobuf:
		// Check if the type implements proto.Message
		msg, ok := v.(proto.Message)
		if !ok {
			return nil, fmt.Errorf("%w: value does not implement proto.Message", ErrMarshalFailed)
		}

		data, err = proto.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("%w (Protobuf): %v", ErrMarshalFailed, err)
		}

	default:
		return nil, fmt.Errorf("%w: format %d", ErrUnknownFormat, format)
	}

	// Prepend format byte
	result := make([]byte, len(data)+1)
	result[0] = byte(format)
	copy(result[1:], data)

	return result, nil
}

// Unmarshal deserializes a payload, automatically detecting the format
// The target v must be a pointer to the appropriate type
func (s *Serializer) Unmarshal(data []byte, v interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("%w: empty payload", ErrUnmarshalFailed)
	}

	format, payload, err := s.DetectFormat(data)
	if err != nil {
		return err
	}

	return s.UnmarshalWithFormat(payload, v, format)
}

// UnmarshalWithFormat deserializes a payload using the specified format
func (s *Serializer) UnmarshalWithFormat(data []byte, v interface{}, format PayloadFormat) error {
	switch format {
	case FormatJSON:
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("%w (JSON): %v", ErrUnmarshalFailed, err)
		}
		return nil

	case FormatProtobuf:
		// Check if the type implements proto.Message
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%w: value does not implement proto.Message", ErrUnmarshalFailed)
		}

		if err := proto.Unmarshal(data, msg); err != nil {
			return fmt.Errorf("%w (Protobuf): %v", ErrUnmarshalFailed, err)
		}
		return nil

	default:
		return fmt.Errorf("%w: format %d", ErrUnknownFormat, format)
	}
}

// DetectFormat detects the serialization format of a payload
// Returns the format and the payload without the format prefix
func (s *Serializer) DetectFormat(data []byte) (PayloadFormat, []byte, error) {
	if len(data) == 0 {
		return FormatJSON, nil, fmt.Errorf("%w: empty payload", ErrUnknownFormat)
	}

	// Check if the first byte is a known format marker
	format := PayloadFormat(data[0])

	switch format {
	case FormatJSON, FormatProtobuf:
		// Known format with prefix
		if len(data) < 2 {
			return format, nil, fmt.Errorf("%w: payload too short", ErrUnmarshalFailed)
		}
		return format, data[1:], nil

	default:
		// Assume legacy JSON without format prefix
		// JSON typically starts with '{' (0x7B) or '[' (0x5B)
		if data[0] == '{' || data[0] == '[' {
			return FormatJSON, data, nil
		}

		// Unknown format
		return FormatJSON, data, fmt.Errorf("%w: unknown format byte 0x%02X", ErrUnknownFormat, data[0])
	}
}

// IsProtobuf returns true if the data is in protobuf format
func (s *Serializer) IsProtobuf(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return PayloadFormat(data[0]) == FormatProtobuf
}

// IsJSON returns true if the data is in JSON format
func (s *Serializer) IsJSON(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	format := PayloadFormat(data[0])
	if format == FormatJSON {
		return true
	}

	// Check for legacy JSON without prefix
	return data[0] == '{' || data[0] == '['
}

// GetFormat returns the format of a serialized payload
func (s *Serializer) GetFormat(data []byte) (PayloadFormat, error) {
	format, _, err := s.DetectFormat(data)
	return format, err
}
