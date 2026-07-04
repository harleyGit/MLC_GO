package VideoUploadServicePackage

import "testing"

func TestVideoListCounterDelta(t *testing.T) {
	tests := []struct {
		name      string
		oldStatus string
		newStatus string
		want      int64
	}{
		{name: "draft to reviewing increments", oldStatus: "draft", newStatus: "reviewing", want: 1},
		{name: "reviewing to reviewing unchanged", oldStatus: "reviewing", newStatus: "reviewing", want: 0},
		{name: "reviewing to draft decrements", oldStatus: "reviewing", newStatus: "draft", want: -1},
		{name: "published to reviewing unchanged", oldStatus: "published", newStatus: "reviewing", want: 0},
		{name: "missing to published increments", oldStatus: "", newStatus: "published", want: 1},
		{name: "missing to draft unchanged", oldStatus: "", newStatus: "draft", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := videoListCounterDelta(tt.oldStatus, tt.newStatus); got != tt.want {
				t.Fatalf("videoListCounterDelta(%q, %q) = %d, want %d", tt.oldStatus, tt.newStatus, got, tt.want)
			}
		})
	}
}

func TestVideoListCounterUpdate(t *testing.T) {
	tests := []struct {
		name       string
		oldStatus  string
		newStatus  string
		wantStatus string
		wantDelta  int64
	}{
		{name: "enter reviewing increments reviewing", oldStatus: "draft", newStatus: "reviewing", wantStatus: "reviewing", wantDelta: 1},
		{name: "leave reviewing decrements reviewing", oldStatus: "reviewing", newStatus: "draft", wantStatus: "reviewing", wantDelta: -1},
		{name: "reviewing to reviewing unchanged", oldStatus: "reviewing", newStatus: "reviewing", wantStatus: "", wantDelta: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotDelta := videoListCounterUpdate(tt.oldStatus, tt.newStatus)
			if gotStatus != tt.wantStatus || gotDelta != tt.wantDelta {
				t.Fatalf("videoListCounterUpdate(%q, %q) = (%q, %d), want (%q, %d)", tt.oldStatus, tt.newStatus, gotStatus, gotDelta, tt.wantStatus, tt.wantDelta)
			}
		})
	}
}

func TestVideoStatusCounterSyncInterval(t *testing.T) {
	if videoStatusCounterSyncInterval <= 0 {
		t.Fatalf("videoStatusCounterSyncInterval must be positive")
	}
}
