package selector

import "testing"

func TestNewModel(t *testing.T) {
	remotes := []Remote{
		{Name: "origin", URL: "git@github.com:user/repo.git"},
		{Name: "upstream", URL: "git@gitlab.com:org/repo.git"},
	}
	m := New(remotes)
	if m.cursor != 0 { t.Errorf("cursor: got %d, want 0", m.cursor) }
	if len(m.remotes) != 2 { t.Errorf("remotes: got %d, want 2", len(m.remotes)) }
}

func TestCursorMovement(t *testing.T) {
	remotes := []Remote{
		{Name: "origin", URL: "url1"},
		{Name: "upstream", URL: "url2"},
		{Name: "fork", URL: "url3"},
	}
	m := New(remotes)
	m.MoveDown()
	if m.cursor != 1 { t.Errorf("cursor: got %d, want 1", m.cursor) }
	m.MoveDown()
	m.MoveDown() // clamp
	if m.cursor != 2 { t.Errorf("cursor: got %d, want 2", m.cursor) }
	m.MoveUp()
	if m.cursor != 1 { t.Errorf("cursor: got %d, want 1", m.cursor) }
}

func TestSelect(t *testing.T) {
	remotes := []Remote{
		{Name: "origin", URL: "url1"},
		{Name: "upstream", URL: "url2"},
	}
	m := New(remotes)
	m.MoveDown()
	m.Select()
	r, ok := m.Selected()
	if !ok || r.Name != "upstream" { t.Errorf("selected: got %q, want %q", r.Name, "upstream") }
}

func TestNotSelectedUntilEnter(t *testing.T) {
	m := New([]Remote{{Name: "origin", URL: "url"}})
	_, ok := m.Selected()
	if ok { t.Error("should not be selected before Enter") }
}
