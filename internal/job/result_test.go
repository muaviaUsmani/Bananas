package job

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJobResult_IsSuccess(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		want   bool
	}{
		{"Completed", StatusCompleted, true},
		{"Failed", StatusFailed, false},
		{"Pending", StatusPending, false},
		{"Processing", StatusProcessing, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &JobResult{Status: tt.status}
			if got := r.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobResult_IsFailed(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		want   bool
	}{
		{"Failed", StatusFailed, true},
		{"Completed", StatusCompleted, false},
		{"Pending", StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &JobResult{Status: tt.status}
			if got := r.IsFailed(); got != tt.want {
				t.Errorf("IsFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobResult_UnmarshalResult(t *testing.T) {
	t.Run("Success with data", func(t *testing.T) {
		data := map[string]interface{}{"count": float64(42), "status": "done"}
		resultBytes, _ := json.Marshal(data)

		r := &JobResult{
			Status: StatusCompleted,
			Result: resultBytes,
		}

		var dest map[string]interface{}
		err := r.UnmarshalResult(&dest)
		if err != nil {
			t.Fatalf("UnmarshalResult() error = %v", err)
		}

		if dest["count"] != float64(42) {
			t.Errorf("count = %v, want 42", dest["count"])
		}
	})

	t.Run("Success with no data", func(t *testing.T) {
		r := &JobResult{
			Status: StatusCompleted,
			Result: nil,
		}

		var dest map[string]interface{}
		err := r.UnmarshalResult(&dest)
		if err != nil {
			t.Fatalf("UnmarshalResult() error = %v", err)
		}
	})

	t.Run("Failed job", func(t *testing.T) {
		r := &JobResult{
			Status: StatusFailed,
			Error:  "something went wrong",
		}

		var dest map[string]interface{}
		err := r.UnmarshalResult(&dest)
		if err == nil {
			t.Fatal("UnmarshalResult() expected error for failed job")
		}

		resultErr, ok := err.(*ResultError)
		if !ok {
			t.Fatalf("error type = %T, want *ResultError", err)
		}

		if resultErr.Message != "something went wrong" {
			t.Errorf("error message = %v, want 'something went wrong'", resultErr.Message)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		r := &JobResult{
			Status: StatusCompleted,
			Result: json.RawMessage("invalid json"),
		}

		var dest map[string]interface{}
		err := r.UnmarshalResult(&dest)
		if err == nil {
			t.Fatal("UnmarshalResult() expected error for invalid JSON")
		}
	})
}

func TestJobResult_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	duration := 5 * time.Second

	r := &JobResult{
		JobID:       "job123",
		Status:      StatusCompleted,
		Result:      json.RawMessage(`{"count":42}`),
		CompletedAt: now,
		Duration:    duration,
	}

	// Marshal
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// Unmarshal
	var r2 JobResult
	err = json.Unmarshal(data, &r2)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if r2.JobID != r.JobID {
		t.Errorf("JobID = %v, want %v", r2.JobID, r.JobID)
	}

	if r2.Status != r.Status {
		t.Errorf("Status = %v, want %v", r2.Status, r.Status)
	}

	if string(r2.Result) != string(r.Result) {
		t.Errorf("Result = %v, want %v", string(r2.Result), string(r.Result))
	}
}
