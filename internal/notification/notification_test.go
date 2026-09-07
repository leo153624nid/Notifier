package notification

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		n       Notification
		wantErr bool
	}{
		{
			name:    "valid notification",
			n:       Notification{Recipient: "some", Channel: "email"},
			wantErr: false,
		},
		{
			name:    "empty recipient",
			n:       Notification{Recipient: "", Channel: "email"},
			wantErr: true,
		},
		{
			name:    "empty channel",
			n:       Notification{Recipient: "some", Channel: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.n.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error: %s", err)
			}
		})
	}
}
