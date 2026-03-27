package confirm

import "testing"

func TestNewDialog(t *testing.T) {
	d := New("Retry this pipeline?", "retry")
	if d.Message != "Retry this pipeline?" {
		t.Errorf("message: got %q", d.Message)
	}
	if d.Action != "retry" {
		t.Errorf("action: got %q", d.Action)
	}
	if d.Confirmed {
		t.Error("should not be confirmed initially")
	}
	if d.Answered {
		t.Error("should not be answered initially")
	}
}

func TestConfirm(t *testing.T) {
	d := New("Cancel?", "cancel")
	d.Confirm()
	if !d.Confirmed || !d.Answered {
		t.Error("should be confirmed and answered")
	}
}

func TestDeny(t *testing.T) {
	d := New("Cancel?", "cancel")
	d.Deny()
	if d.Confirmed {
		t.Error("should not be confirmed")
	}
	if !d.Answered {
		t.Error("should be answered")
	}
}
