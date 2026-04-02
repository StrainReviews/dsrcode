package session_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/session"
)

// TestRegistryStartSession verifies that StartSession adds a new session to
// the registry and GetSession returns it with correct fields.
func TestRegistryStartSession(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req := session.ActivityRequest{
		SessionID: "sess-1",
		Cwd:       "/home/user/project",
	}

	s := reg.StartSession(req, 1234)
	if s == nil {
		t.Fatal("StartSession returned nil")
	}

	if s.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", s.SessionID, "sess-1")
	}
	if s.ProjectPath != "/home/user/project" {
		t.Errorf("ProjectPath = %q, want %q", s.ProjectPath, "/home/user/project")
	}
	if s.ProjectName != "project" {
		t.Errorf("ProjectName = %q, want %q", s.ProjectName, "project")
	}
	if s.PID != 1234 {
		t.Errorf("PID = %d, want %d", s.PID, 1234)
	}
	if s.Status != session.StatusActive {
		t.Errorf("Status = %q, want %q", s.Status, session.StatusActive)
	}
	if s.SmallImageKey != "starting" {
		t.Errorf("SmallImageKey = %q, want %q", s.SmallImageKey, "starting")
	}
	if reg.SessionCount() != 1 {
		t.Errorf("SessionCount = %d, want 1", reg.SessionCount())
	}
	if changed != 1 {
		t.Errorf("onChange called %d times, want 1", changed)
	}

	// GetSession returns a copy with correct data
	got := reg.GetSession("sess-1")
	if got == nil {
		t.Fatal("GetSession returned nil")
	}
	if got.SessionID != "sess-1" {
		t.Errorf("GetSession SessionID = %q, want %q", got.SessionID, "sess-1")
	}
}

// TestRegistryUpdateActivity verifies that UpdateActivity changes the
// SmallImageKey and increments the corresponding ActivityCounts counter.
func TestRegistryUpdateActivity(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req := session.ActivityRequest{
		SessionID: "sess-1",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)
	changed = 0 // reset after start

	updateReq := session.ActivityRequest{
		SessionID:     "sess-1",
		SmallImageKey: "coding",
		SmallText:     "Editing files",
		Details:       "Working on main.go",
	}

	s := reg.UpdateActivity("sess-1", updateReq)
	if s == nil {
		t.Fatal("UpdateActivity returned nil")
	}
	if s.SmallImageKey != "coding" {
		t.Errorf("SmallImageKey = %q, want %q", s.SmallImageKey, "coding")
	}
	if s.SmallText != "Editing files" {
		t.Errorf("SmallText = %q, want %q", s.SmallText, "Editing files")
	}
	if s.Details != "Working on main.go" {
		t.Errorf("Details = %q, want %q", s.Details, "Working on main.go")
	}
	if s.ActivityCounts.Edits != 1 {
		t.Errorf("Edits = %d, want 1", s.ActivityCounts.Edits)
	}
	if s.Status != session.StatusActive {
		t.Errorf("Status = %q, want %q", s.Status, session.StatusActive)
	}
	if changed != 1 {
		t.Errorf("onChange called %d times, want 1", changed)
	}

	// Update again with terminal activity
	termReq := session.ActivityRequest{
		SessionID:     "sess-1",
		SmallImageKey: "terminal",
		SmallText:     "Running command",
	}
	s = reg.UpdateActivity("sess-1", termReq)
	if s.ActivityCounts.Commands != 1 {
		t.Errorf("Commands = %d, want 1", s.ActivityCounts.Commands)
	}
	if s.ActivityCounts.Edits != 1 {
		t.Errorf("Edits after terminal update = %d, want 1 (unchanged)", s.ActivityCounts.Edits)
	}

	// Update non-existent session returns nil
	if got := reg.UpdateActivity("nonexistent", updateReq); got != nil {
		t.Error("UpdateActivity on nonexistent session should return nil")
	}
}

// TestRegistryEndSession verifies that EndSession removes the session from the
// registry so that subsequent GetSession calls return nil.
func TestRegistryEndSession(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req := session.ActivityRequest{
		SessionID: "sess-1",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)
	changed = 0

	reg.EndSession("sess-1")

	if reg.SessionCount() != 0 {
		t.Errorf("SessionCount = %d, want 0 after EndSession", reg.SessionCount())
	}
	if got := reg.GetSession("sess-1"); got != nil {
		t.Error("GetSession should return nil after EndSession")
	}
	if changed != 1 {
		t.Errorf("onChange called %d times, want 1", changed)
	}

	// EndSession on nonexistent session does not call onChange
	changed = 0
	reg.EndSession("nonexistent")
	if changed != 0 {
		t.Errorf("onChange called %d times for nonexistent EndSession, want 0", changed)
	}
}

// TestRegistrySessionDedup verifies that two StartSession calls with the same
// projectPath and pid return the same session rather than creating duplicates.
func TestRegistrySessionDedup(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req1 := session.ActivityRequest{
		SessionID: "sess-1",
		Cwd:       "/home/user/project",
	}
	s1 := reg.StartSession(req1, 1234)

	// Same Cwd + PID, different SessionID
	req2 := session.ActivityRequest{
		SessionID: "sess-2",
		Cwd:       "/home/user/project",
	}
	s2 := reg.StartSession(req2, 1234)

	// Should return the existing session, not create a new one
	if s1.SessionID != s2.SessionID {
		t.Errorf("Dedup failed: got sessionIDs %q and %q, expected same", s1.SessionID, s2.SessionID)
	}
	if reg.SessionCount() != 1 {
		t.Errorf("SessionCount = %d, want 1 (deduped)", reg.SessionCount())
	}
	// onChange should only have been called once (for the first StartSession)
	if changed != 1 {
		t.Errorf("onChange called %d times, want 1 (dedup should not trigger)", changed)
	}

	// Different PID, same Cwd: should create a new session
	req3 := session.ActivityRequest{
		SessionID: "sess-3",
		Cwd:       "/home/user/project",
	}
	s3 := reg.StartSession(req3, 5678)
	if s3.SessionID == s1.SessionID {
		t.Error("Different PID should create a new session")
	}
	if reg.SessionCount() != 2 {
		t.Errorf("SessionCount = %d, want 2", reg.SessionCount())
	}
}

// TestRegistryConcurrentAccess verifies that 100 goroutines can read and write
// to the registry simultaneously without data races (test with -race flag).
func TestRegistryConcurrentAccess(t *testing.T) {
	reg := session.NewRegistry(func() {})

	var wg sync.WaitGroup
	const goroutines = 100

	// Half write, half read
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := "sess-concurrent"
			req := session.ActivityRequest{
				SessionID: sid,
				Cwd:       "/home/user/project",
			}
			if n%2 == 0 {
				reg.StartSession(req, n)
				reg.UpdateActivity(sid, session.ActivityRequest{
					SessionID:     sid,
					SmallImageKey: "coding",
				})
				reg.UpdateSessionData(sid, "opus", "main", 1000, 0.05)
			} else {
				reg.GetSession(sid)
				reg.GetAllSessions()
				reg.SessionCount()
				reg.GetMostRecentSession()
			}
		}(i)
	}

	wg.Wait()
	// If we get here without race detector panic, the test passes
}

// TestStaleCheck verifies that a session whose LastActivityAt exceeds the
// idleTimeout gets its Status set to Idle.
func TestStaleCheck(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "sess-stale",
		Cwd:       "/home/user/project",
	}
	// Use current PID so IsPidAlive returns true (we don't want PID-death removal)
	reg.StartSession(req, os.Getpid())

	// Set LastActivityAt to 11 minutes ago
	reg.SetLastActivityForTest("sess-stale", time.Now().Add(-11*time.Minute))

	// Run stale check with 10min idle timeout, 30min remove timeout
	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	s := reg.GetSession("sess-stale")
	if s == nil {
		t.Fatal("session should still exist (only idle, not removed)")
	}
	if s.Status != session.StatusIdle {
		t.Errorf("Status = %q, want %q", s.Status, session.StatusIdle)
	}
	if s.SmallImageKey != "idle" {
		t.Errorf("SmallImageKey = %q, want %q", s.SmallImageKey, "idle")
	}
}

// TestStaleRemove verifies that a session whose LastActivityAt exceeds the
// removeTimeout gets removed from the registry entirely.
func TestStaleRemove(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "sess-remove",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, os.Getpid())

	// Set LastActivityAt to 31 minutes ago
	reg.SetLastActivityForTest("sess-remove", time.Now().Add(-31*time.Minute))

	// Run stale check with 10min idle timeout, 30min remove timeout
	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	s := reg.GetSession("sess-remove")
	if s != nil {
		t.Error("session should have been removed after exceeding removeTimeout")
	}
	if reg.SessionCount() != 0 {
		t.Errorf("SessionCount = %d, want 0 after removal", reg.SessionCount())
	}
}

// TestPidLiveness verifies that IsPidAlive returns true for the current
// process (os.Getpid()) and false for a non-existent PID (99999999).
func TestPidLiveness(t *testing.T) {
	// Current process should be alive
	if !session.IsPidAlive(os.Getpid()) {
		t.Error("IsPidAlive(os.Getpid()) = false, want true")
	}

	// Non-existent PID should not be alive
	if session.IsPidAlive(99999999) {
		t.Error("IsPidAlive(99999999) = true, want false")
	}
}
