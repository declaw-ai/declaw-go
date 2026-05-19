package declaw

import (
	"testing"
	"time"
)

// --- SandboxState constants ---

func TestSandboxState_Constants(t *testing.T) {
	tests := []struct {
		name     string
		state    SandboxState
		expected string
	}{
		{"StateLive", StateLive, "live"},
		{"StatePaused", StatePaused, "paused"},
		{"StateKilled", StateKilled, "killed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.state) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.state, tc.expected)
			}
		})
	}
}

func TestSandboxState_IsStringType(t *testing.T) {
	// SandboxState should be castable to/from string
	var s SandboxState = "custom"
	if string(s) != "custom" {
		t.Errorf("expected custom state, got %q", s)
	}
}

// --- FileType constants ---

func TestFileType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		ft       FileType
		expected string
	}{
		{"FileTypeFile", FileTypeFile, "file"},
		{"FileTypeDirectory", FileTypeDirectory, "dir"},
		{"FileTypeSymlink", FileTypeSymlink, "symlink"},
		{"FileTypeOther", FileTypeOther, "other"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.ft) != tc.expected {
				t.Errorf("%s = %q, want %q", tc.name, tc.ft, tc.expected)
			}
		})
	}
}

// --- SandboxInfo ---

func TestSandboxInfo_AllFields(t *testing.T) {
	now := time.Now()
	later := now.Add(1 * time.Hour)

	info := SandboxInfo{
		SandboxID:  "sbx-abc123",
		TemplateID: "tpl-python",
		Name:       "my-sandbox",
		Metadata:   map[string]string{"env": "test", "owner": "alice"},
		StartedAt:  &now,
		EndAt:      &later,
		State:      StateLive,
	}

	if info.SandboxID != "sbx-abc123" {
		t.Errorf("SandboxID = %q, want %q", info.SandboxID, "sbx-abc123")
	}
	if info.TemplateID != "tpl-python" {
		t.Errorf("TemplateID = %q, want %q", info.TemplateID, "tpl-python")
	}
	if info.Name != "my-sandbox" {
		t.Errorf("Name = %q, want %q", info.Name, "my-sandbox")
	}
	if info.Metadata["env"] != "test" {
		t.Errorf("Metadata[env] = %q, want %q", info.Metadata["env"], "test")
	}
	if info.Metadata["owner"] != "alice" {
		t.Errorf("Metadata[owner] = %q, want %q", info.Metadata["owner"], "alice")
	}
	if !info.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", info.StartedAt, now)
	}
	if !info.EndAt.Equal(later) {
		t.Errorf("EndAt = %v, want %v", info.EndAt, later)
	}
	if info.State != StateLive {
		t.Errorf("State = %q, want %q", info.State, StateLive)
	}
}

func TestSandboxInfo_ZeroValue(t *testing.T) {
	var info SandboxInfo

	if info.SandboxID != "" {
		t.Errorf("zero SandboxID = %q, want empty", info.SandboxID)
	}
	if info.TemplateID != "" {
		t.Errorf("zero TemplateID = %q, want empty", info.TemplateID)
	}
	if info.Name != "" {
		t.Errorf("zero Name = %q, want empty", info.Name)
	}
	if info.Metadata != nil {
		t.Errorf("zero Metadata should be nil, got %v", info.Metadata)
	}
	if info.StartedAt != nil {
		t.Errorf("zero StartedAt should be nil, got %v", info.StartedAt)
	}
	if info.EndAt != nil {
		t.Errorf("zero EndAt should be nil, got %v", info.EndAt)
	}
	if info.State != "" {
		t.Errorf("zero State = %q, want empty", info.State)
	}
}

func TestSandboxInfo_NilOptionalFields(t *testing.T) {
	info := SandboxInfo{
		SandboxID: "sbx-1",
		State:     StateLive,
	}

	if info.StartedAt != nil {
		t.Error("StartedAt should be nil when not set")
	}
	if info.EndAt != nil {
		t.Error("EndAt should be nil when not set")
	}
	if info.Metadata != nil {
		t.Error("Metadata should be nil when not set")
	}
}

// --- SandboxMetrics ---

func TestSandboxMetrics_AllFields(t *testing.T) {
	now := time.Now()
	m := SandboxMetrics{
		Timestamp:       now,
		CPUUsagePercent: 42.5,
		MemoryUsageMB:   512.0,
		DiskUsageMB:     1024.0,
	}

	if !m.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", m.Timestamp, now)
	}
	if m.CPUUsagePercent != 42.5 {
		t.Errorf("CPUUsagePercent = %f, want 42.5", m.CPUUsagePercent)
	}
	if m.MemoryUsageMB != 512.0 {
		t.Errorf("MemoryUsageMB = %f, want 512.0", m.MemoryUsageMB)
	}
	if m.DiskUsageMB != 1024.0 {
		t.Errorf("DiskUsageMB = %f, want 1024.0", m.DiskUsageMB)
	}
}

func TestSandboxMetrics_ZeroValues(t *testing.T) {
	var m SandboxMetrics

	if m.CPUUsagePercent != 0 {
		t.Errorf("zero CPUUsagePercent = %f, want 0", m.CPUUsagePercent)
	}
	if m.MemoryUsageMB != 0 {
		t.Errorf("zero MemoryUsageMB = %f, want 0", m.MemoryUsageMB)
	}
	if m.DiskUsageMB != 0 {
		t.Errorf("zero DiskUsageMB = %f, want 0", m.DiskUsageMB)
	}
}

// --- CommandResult ---

func TestCommandResult_AllFields(t *testing.T) {
	r := CommandResult{
		PID:      1234,
		ExitCode: 0,
		Stdout:   "hello world\n",
		Stderr:   "",
	}

	if r.PID != 1234 {
		t.Errorf("PID = %d, want 1234", r.PID)
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
	if r.Stdout != "hello world\n" {
		t.Errorf("Stdout = %q, want %q", r.Stdout, "hello world\n")
	}
	if r.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", r.Stderr)
	}
}

func TestCommandResult_NonZeroExit(t *testing.T) {
	r := CommandResult{
		PID:      42,
		ExitCode: 127,
		Stdout:   "",
		Stderr:   "command not found",
	}

	if r.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127", r.ExitCode)
	}
	if r.Stderr != "command not found" {
		t.Errorf("Stderr = %q, want %q", r.Stderr, "command not found")
	}
}

func TestCommandResult_ZeroValue(t *testing.T) {
	var r CommandResult

	if r.PID != 0 {
		t.Errorf("zero PID = %d, want 0", r.PID)
	}
	if r.ExitCode != 0 {
		t.Errorf("zero ExitCode = %d, want 0", r.ExitCode)
	}
	if r.Stdout != "" {
		t.Errorf("zero Stdout = %q, want empty", r.Stdout)
	}
	if r.Stderr != "" {
		t.Errorf("zero Stderr = %q, want empty", r.Stderr)
	}
}

// --- ProcessInfo ---

func TestProcessInfo_AllFields(t *testing.T) {
	p := ProcessInfo{
		PID:   9876,
		Cmd:   "/usr/bin/python3 main.py",
		IsPty: true,
		Envs:  map[string]string{"HOME": "/root"},
	}

	if p.PID != 9876 {
		t.Errorf("PID = %d, want 9876", p.PID)
	}
	if p.Cmd != "/usr/bin/python3 main.py" {
		t.Errorf("Cmd = %q, want expected", p.Cmd)
	}
	if !p.IsPty {
		t.Error("IsPty = false, want true")
	}
	if p.Envs["HOME"] != "/root" {
		t.Errorf("Envs[HOME] = %q, want /root", p.Envs["HOME"])
	}
}

func TestProcessInfo_ZeroValue(t *testing.T) {
	var p ProcessInfo

	if p.PID != 0 {
		t.Errorf("zero PID = %d, want 0", p.PID)
	}
	if p.Cmd != "" {
		t.Errorf("zero Cmd = %q, want empty", p.Cmd)
	}
	if p.IsPty {
		t.Error("zero IsPty should be false")
	}
}

// --- EntryInfo ---

func TestEntryInfo_FullConstruction(t *testing.T) {
	e := EntryInfo{
		Path: "/home/user/file.txt",
		Type: FileTypeFile,
		Size: 4096,
	}

	if e.Path != "/home/user/file.txt" {
		t.Errorf("Path = %q, want expected", e.Path)
	}
	if e.Type != FileTypeFile {
		t.Errorf("Type = %q, want %q", e.Type, FileTypeFile)
	}
	if e.Size != 4096 {
		t.Errorf("Size = %d, want 4096", e.Size)
	}
}

func TestEntryInfo_Directory(t *testing.T) {
	e := EntryInfo{
		Path: "/home/user/",
		Type: FileTypeDirectory,
	}

	if e.Type != FileTypeDirectory {
		t.Errorf("Type = %q, want %q", e.Type, FileTypeDirectory)
	}
}

func TestEntryInfo_Symlink(t *testing.T) {
	e := EntryInfo{
		Path: "/usr/bin/python",
		Type: FileTypeSymlink,
	}

	if e.Type != FileTypeSymlink {
		t.Errorf("Type = %q, want %q", e.Type, FileTypeSymlink)
	}
}

func TestEntryInfo_ZeroValue(t *testing.T) {
	var e EntryInfo

	if e.Path != "" {
		t.Errorf("zero Path = %q, want empty", e.Path)
	}
	if e.Type != "" {
		t.Errorf("zero Type = %q, want empty", e.Type)
	}
	if e.Size != 0 {
		t.Errorf("zero Size = %d, want 0", e.Size)
	}
}

// --- WriteInfo ---

func TestWriteInfo_AllFields(t *testing.T) {
	w := WriteInfo{
		Path: "/tmp/output.log",
		Size: 256,
	}

	if w.Path != "/tmp/output.log" {
		t.Errorf("Path = %q, want expected", w.Path)
	}
	if w.Size != 256 {
		t.Errorf("Size = %d, want 256", w.Size)
	}
}

func TestWriteInfo_ZeroValue(t *testing.T) {
	var w WriteInfo

	if w.Path != "" {
		t.Errorf("zero Path = %q, want empty", w.Path)
	}
	if w.Size != 0 {
		t.Errorf("zero Size = %d, want 0", w.Size)
	}
}

// --- SnapshotInfo ---

func TestSnapshotInfo_AllFields(t *testing.T) {
	now := time.Now()
	s := SnapshotInfo{
		SnapshotID: "snap-abc",
		SandboxID:  "sbx-123",
		CreatedAt:  &now,
	}

	if s.SnapshotID != "snap-abc" {
		t.Errorf("SnapshotID = %q, want %q", s.SnapshotID, "snap-abc")
	}
	if s.SandboxID != "sbx-123" {
		t.Errorf("SandboxID = %q, want %q", s.SandboxID, "sbx-123")
	}
	if !s.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", s.CreatedAt, now)
	}
}

func TestSnapshotInfo_NilCreatedAt(t *testing.T) {
	s := SnapshotInfo{
		SnapshotID: "snap-1",
		SandboxID:  "sbx-1",
	}

	if s.CreatedAt != nil {
		t.Errorf("CreatedAt should be nil when not set, got %v", s.CreatedAt)
	}
}

func TestSnapshotInfo_ZeroValue(t *testing.T) {
	var s SnapshotInfo

	if s.SnapshotID != "" {
		t.Errorf("zero SnapshotID = %q, want empty", s.SnapshotID)
	}
	if s.SandboxID != "" {
		t.Errorf("zero SandboxID = %q, want empty", s.SandboxID)
	}
	if s.CreatedAt != nil {
		t.Errorf("zero CreatedAt should be nil")
	}
}

// --- Snapshot ---

func TestSnapshot_AllFields(t *testing.T) {
	memSize := int64(536870912)
	pauseMs := int64(150)

	snap := Snapshot{
		SnapshotID:      "snap-full",
		SandboxID:       "sbx-full",
		Source:          "pause",
		MemBlobKey:      "mem-key-abc",
		VMStateBlobKey:  "vmstate-key-xyz",
		MemSizeBytes:    &memSize,
		PauseDurationMs: &pauseMs,
		CreatedAt:       "2026-05-19T12:00:00Z",
	}

	if snap.SnapshotID != "snap-full" {
		t.Errorf("SnapshotID = %q, want %q", snap.SnapshotID, "snap-full")
	}
	if snap.Source != "pause" {
		t.Errorf("Source = %q, want %q", snap.Source, "pause")
	}
	if snap.MemBlobKey != "mem-key-abc" {
		t.Errorf("MemBlobKey = %q, want expected", snap.MemBlobKey)
	}
	if snap.VMStateBlobKey != "vmstate-key-xyz" {
		t.Errorf("VMStateBlobKey = %q, want expected", snap.VMStateBlobKey)
	}
	if *snap.MemSizeBytes != 536870912 {
		t.Errorf("MemSizeBytes = %d, want 536870912", *snap.MemSizeBytes)
	}
	if *snap.PauseDurationMs != 150 {
		t.Errorf("PauseDurationMs = %d, want 150", *snap.PauseDurationMs)
	}
	if snap.CreatedAt != "2026-05-19T12:00:00Z" {
		t.Errorf("CreatedAt = %q, want expected", snap.CreatedAt)
	}
}

func TestSnapshot_NilOptionalFields(t *testing.T) {
	snap := Snapshot{
		SnapshotID: "snap-minimal",
		SandboxID:  "sbx-1",
		Source:     "manual",
	}

	if snap.MemSizeBytes != nil {
		t.Errorf("MemSizeBytes should be nil, got %v", snap.MemSizeBytes)
	}
	if snap.PauseDurationMs != nil {
		t.Errorf("PauseDurationMs should be nil, got %v", snap.PauseDurationMs)
	}
}

func TestSnapshot_SourceValues(t *testing.T) {
	sources := []string{"periodic", "pause", "manual"}
	for _, src := range sources {
		t.Run(src, func(t *testing.T) {
			snap := Snapshot{Source: src}
			if snap.Source != src {
				t.Errorf("Source = %q, want %q", snap.Source, src)
			}
		})
	}
}

// --- PtySize ---

func TestPtySize_AllFields(t *testing.T) {
	ps := PtySize{Cols: 120, Rows: 40}

	if ps.Cols != 120 {
		t.Errorf("Cols = %d, want 120", ps.Cols)
	}
	if ps.Rows != 40 {
		t.Errorf("Rows = %d, want 40", ps.Rows)
	}
}

func TestPtySize_ZeroValue(t *testing.T) {
	var ps PtySize

	if ps.Cols != 0 {
		t.Errorf("zero Cols = %d, want 0", ps.Cols)
	}
	if ps.Rows != 0 {
		t.Errorf("zero Rows = %d, want 0", ps.Rows)
	}
}

// --- SandboxLifecycle ---

func TestSandboxLifecycle_Kill(t *testing.T) {
	lc := SandboxLifecycle{
		OnTimeout:  "kill",
		AutoResume: false,
	}

	if lc.OnTimeout != "kill" {
		t.Errorf("OnTimeout = %q, want %q", lc.OnTimeout, "kill")
	}
	if lc.AutoResume != false {
		t.Errorf("AutoResume = %v, want false", lc.AutoResume)
	}
}

func TestSandboxLifecycle_Pause(t *testing.T) {
	lc := SandboxLifecycle{
		OnTimeout:  "pause",
		AutoResume: true,
	}

	if lc.OnTimeout != "pause" {
		t.Errorf("OnTimeout = %q, want %q", lc.OnTimeout, "pause")
	}
	if lc.AutoResume != true {
		t.Errorf("AutoResume = %v, want true", lc.AutoResume)
	}
}

// --- VolumeAttachment ---

func TestVolumeAttachment_AllFields(t *testing.T) {
	va := VolumeAttachment{
		VolumeID:  "vol-abc",
		MountPath: "/mnt/data",
	}

	if va.VolumeID != "vol-abc" {
		t.Errorf("VolumeID = %q, want expected", va.VolumeID)
	}
	if va.MountPath != "/mnt/data" {
		t.Errorf("MountPath = %q, want expected", va.MountPath)
	}
}

// --- VolumeInfo ---

func TestVolumeInfo_AllFields(t *testing.T) {
	vi := VolumeInfo{
		VolumeID:    "vol-xyz",
		OwnerID:     "user-123",
		Name:        "my-volume",
		BlobKey:     "blob-key-abc",
		SizeBytes:   1073741824,
		ContentType: "application/octet-stream",
		CreatedAt:   "2026-05-19T12:00:00Z",
		Metadata:    map[string]string{"tier": "ssd"},
	}

	if vi.VolumeID != "vol-xyz" {
		t.Errorf("VolumeID = %q, want expected", vi.VolumeID)
	}
	if vi.OwnerID != "user-123" {
		t.Errorf("OwnerID = %q, want expected", vi.OwnerID)
	}
	if vi.Name != "my-volume" {
		t.Errorf("Name = %q, want expected", vi.Name)
	}
	if vi.BlobKey != "blob-key-abc" {
		t.Errorf("BlobKey = %q, want expected", vi.BlobKey)
	}
	if vi.SizeBytes != 1073741824 {
		t.Errorf("SizeBytes = %d, want 1073741824", vi.SizeBytes)
	}
	if vi.ContentType != "application/octet-stream" {
		t.Errorf("ContentType = %q, want expected", vi.ContentType)
	}
	if vi.CreatedAt != "2026-05-19T12:00:00Z" {
		t.Errorf("CreatedAt = %q, want expected", vi.CreatedAt)
	}
	if vi.Metadata["tier"] != "ssd" {
		t.Errorf("Metadata[tier] = %q, want %q", vi.Metadata["tier"], "ssd")
	}
}

func TestVolumeInfo_ZeroValue(t *testing.T) {
	var vi VolumeInfo

	if vi.VolumeID != "" {
		t.Errorf("zero VolumeID = %q, want empty", vi.VolumeID)
	}
	if vi.SizeBytes != 0 {
		t.Errorf("zero SizeBytes = %d, want 0", vi.SizeBytes)
	}
	if vi.Metadata != nil {
		t.Errorf("zero Metadata should be nil")
	}
}

// --- TemplateSpec ---

func TestTemplateSpec_AllFields(t *testing.T) {
	ts := TemplateSpec{
		BaseImage:   "python:3.12",
		RunCmds:     []string{"pip install flask", "mkdir /app"},
		Copies:      []CopyItem{{Src: "app.py", Dst: "/app/app.py", Mode: "0644"}},
		Envs:        map[string]string{"FLASK_ENV": "production"},
		AptPackages: []string{"curl", "git"},
		StartCmd:    "python /app/app.py",
		Dockerfile:  "",
		DiskMB:      2048,
	}

	if ts.BaseImage != "python:3.12" {
		t.Errorf("BaseImage = %q, want expected", ts.BaseImage)
	}
	if len(ts.RunCmds) != 2 {
		t.Errorf("RunCmds length = %d, want 2", len(ts.RunCmds))
	}
	if ts.RunCmds[0] != "pip install flask" {
		t.Errorf("RunCmds[0] = %q, want expected", ts.RunCmds[0])
	}
	if len(ts.Copies) != 1 {
		t.Errorf("Copies length = %d, want 1", len(ts.Copies))
	}
	if ts.Copies[0].Src != "app.py" {
		t.Errorf("Copies[0].Src = %q, want expected", ts.Copies[0].Src)
	}
	if ts.Copies[0].Dst != "/app/app.py" {
		t.Errorf("Copies[0].Dst = %q, want expected", ts.Copies[0].Dst)
	}
	if ts.Copies[0].Mode != "0644" {
		t.Errorf("Copies[0].Mode = %q, want expected", ts.Copies[0].Mode)
	}
	if ts.Envs["FLASK_ENV"] != "production" {
		t.Errorf("Envs[FLASK_ENV] = %q, want %q", ts.Envs["FLASK_ENV"], "production")
	}
	if len(ts.AptPackages) != 2 {
		t.Errorf("AptPackages length = %d, want 2", len(ts.AptPackages))
	}
	if ts.StartCmd != "python /app/app.py" {
		t.Errorf("StartCmd = %q, want expected", ts.StartCmd)
	}
	if ts.DiskMB != 2048 {
		t.Errorf("DiskMB = %d, want 2048", ts.DiskMB)
	}
}

func TestTemplateSpec_DockerfileOverride(t *testing.T) {
	ts := TemplateSpec{
		Dockerfile: "FROM python:3.12\nRUN pip install flask\n",
	}

	if ts.Dockerfile == "" {
		t.Error("Dockerfile should not be empty")
	}
}

func TestTemplateSpec_ZeroValueDefaults(t *testing.T) {
	var ts TemplateSpec

	if ts.BaseImage != "" {
		t.Errorf("zero BaseImage = %q, want empty", ts.BaseImage)
	}
	if ts.RunCmds != nil {
		t.Errorf("zero RunCmds should be nil")
	}
	if ts.DiskMB != 0 {
		t.Errorf("zero DiskMB = %d, want 0", ts.DiskMB)
	}
}

// --- CopyItem ---

func TestCopyItem_AllFields(t *testing.T) {
	c := CopyItem{
		Src:  "./local/file.txt",
		Dst:  "/sandbox/file.txt",
		Mode: "0755",
	}

	if c.Src != "./local/file.txt" {
		t.Errorf("Src = %q, want expected", c.Src)
	}
	if c.Dst != "/sandbox/file.txt" {
		t.Errorf("Dst = %q, want expected", c.Dst)
	}
	if c.Mode != "0755" {
		t.Errorf("Mode = %q, want expected", c.Mode)
	}
}

// --- BuildInfo ---

func TestBuildInfo_AllFields(t *testing.T) {
	bi := BuildInfo{
		BuildID:    "build-123",
		Status:     "success",
		TemplateID: "tpl-abc",
	}

	if bi.BuildID != "build-123" {
		t.Errorf("BuildID = %q, want expected", bi.BuildID)
	}
	if bi.Status != "success" {
		t.Errorf("Status = %q, want expected", bi.Status)
	}
	if bi.TemplateID != "tpl-abc" {
		t.Errorf("TemplateID = %q, want expected", bi.TemplateID)
	}
}

func TestBuildInfo_StatusValues(t *testing.T) {
	statuses := []string{"queued", "building", "success", "failed"}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			bi := BuildInfo{Status: status}
			if bi.Status != status {
				t.Errorf("Status = %q, want %q", bi.Status, status)
			}
		})
	}
}

// --- TemplateInfo ---

func TestTemplateInfo_AllFields(t *testing.T) {
	ti := TemplateInfo{
		TemplateID: "tpl-custom",
		Alias:      "my-template",
		CreatedAt:  "2026-05-19T12:00:00Z",
	}

	if ti.TemplateID != "tpl-custom" {
		t.Errorf("TemplateID = %q, want expected", ti.TemplateID)
	}
	if ti.Alias != "my-template" {
		t.Errorf("Alias = %q, want expected", ti.Alias)
	}
	if ti.CreatedAt != "2026-05-19T12:00:00Z" {
		t.Errorf("CreatedAt = %q, want expected", ti.CreatedAt)
	}
}

// --- SandboxPage ---

func TestSandboxPage_WithItems(t *testing.T) {
	page := SandboxPage{
		Sandboxes: []SandboxInfo{
			{SandboxID: "sbx-1", State: StateLive},
			{SandboxID: "sbx-2", State: StatePaused},
		},
		Total: 10,
	}

	if len(page.Sandboxes) != 2 {
		t.Errorf("Sandboxes length = %d, want 2", len(page.Sandboxes))
	}
	if page.Sandboxes[0].SandboxID != "sbx-1" {
		t.Errorf("Sandboxes[0].SandboxID = %q, want %q", page.Sandboxes[0].SandboxID, "sbx-1")
	}
	if page.Total != 10 {
		t.Errorf("Total = %d, want 10", page.Total)
	}
}

func TestSandboxPage_Empty(t *testing.T) {
	page := SandboxPage{}

	if page.Sandboxes != nil {
		t.Errorf("empty Sandboxes should be nil, got %v", page.Sandboxes)
	}
	if page.Total != 0 {
		t.Errorf("empty Total = %d, want 0", page.Total)
	}
}

// --- SnapshotPage ---

func TestSnapshotPage_WithItems(t *testing.T) {
	now := time.Now()
	page := SnapshotPage{
		Snapshots: []SnapshotInfo{
			{SnapshotID: "snap-1", SandboxID: "sbx-1", CreatedAt: &now},
			{SnapshotID: "snap-2", SandboxID: "sbx-1"},
		},
	}

	if len(page.Snapshots) != 2 {
		t.Errorf("Snapshots length = %d, want 2", len(page.Snapshots))
	}
	if page.Snapshots[0].SnapshotID != "snap-1" {
		t.Errorf("Snapshots[0].SnapshotID = %q, want %q", page.Snapshots[0].SnapshotID, "snap-1")
	}
}

func TestSnapshotPage_Empty(t *testing.T) {
	page := SnapshotPage{}

	if page.Snapshots != nil {
		t.Errorf("empty Snapshots should be nil")
	}
}

// --- KillResult ---

func TestKillResult_Success(t *testing.T) {
	kr := KillResult{
		SandboxID: "sbx-1",
		Error:     nil,
	}

	if kr.SandboxID != "sbx-1" {
		t.Errorf("SandboxID = %q, want expected", kr.SandboxID)
	}
	if kr.Error != nil {
		t.Errorf("Error should be nil for success, got %v", kr.Error)
	}
}

func TestKillResult_Failure(t *testing.T) {
	kr := KillResult{
		SandboxID: "sbx-2",
		Error:     &SandboxError{Message: "not found"},
	}

	if kr.Error == nil {
		t.Error("Error should not be nil for failure")
	}
}
