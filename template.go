package declaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Template provides operations for managing sandbox templates.
// Templates define the base environment (packages, files, startup commands)
// for sandbox instances.
type Template struct{}

// Build states reported by the API. A build starts in BuildStatusBuilding and
// ends in one of the other two.
const (
	BuildStatusBuilding  = "building"
	BuildStatusCompleted = "completed"
	BuildStatusFailed    = "failed"
)

// Vars so tests can shorten them.
var (
	// buildPollInterval is how often a waiting build is re-read.
	buildPollInterval = 3 * time.Second

	// buildPollErrorWindow is how long status checks may keep failing
	// temporarily (5xx, 408, 429, no response) before the wait gives up. It
	// rides out a sandbox-manager restart; the build itself keeps running.
	buildPollErrorWindow = 2 * time.Minute
)

// buildFailureLogLines bounds how much build output a BuildError message quotes.
const buildFailureLogLines = 20

// buildLogTruncationNotice is the line sandbox-manager puts first once it starts
// dropping a build's oldest output (store.BuildLogTruncationNotice). It must
// match exactly: it is how logCursor tells a trimmed log from a growing one.
const buildLogTruncationNotice = "... [earlier build output truncated]"

// logAnchorLines is how many of the newest delivered lines logCursor looks for
// in a trimmed log to find where it left off.
const logAnchorLines = 64

// logCursor hands out each line of a build's log once across repeated polls.
//
// The server keeps a bounded number of lines. Until it hits the bound the log
// only grows, and the new lines are those past the last length seen. After, it
// drops the oldest lines and puts buildLogTruncationNotice first, so the length
// stops changing and positions shift between polls; the cursor then finds the
// newest lines it already handed out and continues after them. If none of those
// is left — more output arrived between two polls than the server keeps — the
// gap is marked by handing out the notice itself, followed by what remains.
type logCursor struct {
	seen int
	tail []string
}

// next returns the lines of logs not handed out before.
func (c *logCursor) next(logs []string) []string {
	var fresh []string
	if len(logs) == 0 || logs[0] != buildLogTruncationNotice {
		if c.seen < len(logs) {
			fresh = logs[c.seen:]
		}
	} else {
		fresh = c.afterTail(logs)
	}
	c.seen = len(logs)

	tail := append(append(make([]string, 0, len(c.tail)+len(fresh)), c.tail...), fresh...)
	if len(tail) > logAnchorLines {
		tail = tail[len(tail)-logAnchorLines:]
	}
	c.tail = tail
	return fresh
}

// afterTail returns what follows the newest occurrence of the lines last handed
// out in a trimmed log, or the whole log (notice first) if they are gone.
func (c *logCursor) afterTail(logs []string) []string {
	window := logs[1:]
	n := len(c.tail)
	if n > 0 {
		for end := len(window); end >= n; end-- {
			if window[end-1] == c.tail[n-1] && equalLines(window[end-n:end], c.tail) {
				return window[end:]
			}
		}
	}
	return logs
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// isTemporaryPollError reports whether a failed status check is worth
// repeating: the server or the network hiccuped (5xx, 408, 429, or no response
// at all), as opposed to the API rejecting the request, which waiting will not
// change.
func isTemporaryPollError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr *SandboxError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		return apiErr.StatusCode == http.StatusRequestTimeout || apiErr.StatusCode == http.StatusTooManyRequests
	}
	return true
}

// buildTemplateRequest builds the POST /templates/build body. The server reads
// the spec from a nested "template" object under its own field names
// ("packages", not "apt_packages"), with the required alias and disk_mb beside
// it. It ignores fields it does not know, so a misnamed field is a silent
// no-op rather than an error.
func buildTemplateRequest(spec TemplateSpec) (map[string]interface{}, error) {
	if len(spec.Copies) > 0 {
		return nil, &InvalidArgumentError{SandboxError: &SandboxError{
			Message: "TemplateSpec.Copies is not supported yet: a template build cannot upload " +
				"local files. Fetch them in a RunCmds step, or use a Dockerfile.",
		}}
	}

	tpl := map[string]interface{}{}
	if spec.BaseImage != "" {
		tpl["base_image"] = spec.BaseImage
	}
	if len(spec.RunCmds) > 0 {
		tpl["run_cmds"] = spec.RunCmds
	}
	if len(spec.Envs) > 0 {
		tpl["envs"] = spec.Envs
	}
	if len(spec.AptPackages) > 0 {
		tpl["packages"] = spec.AptPackages
	}
	if spec.StartCmd != "" {
		tpl["start_cmd"] = spec.StartCmd
	}
	if spec.Dockerfile != "" {
		tpl["dockerfile"] = spec.Dockerfile
	}

	body := map[string]interface{}{
		"template": tpl,
		"alias":    spec.Alias,
	}
	if spec.DiskMB > 0 {
		body["disk_mb"] = spec.DiskMB
	}
	return body, nil
}

// parseBuildInfo parses a BuildInfo from a JSON response body.
func parseBuildInfo(data []byte) (*BuildInfo, error) {
	var raw struct {
		BuildID    string   `json:"build_id"`
		Status     string   `json:"status"`
		TemplateID string   `json:"template_id"`
		Logs       []string `json:"logs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing build info: %w", err)
	}
	return &BuildInfo{
		BuildID:    raw.BuildID,
		Status:     raw.Status,
		TemplateID: raw.TemplateID,
		Logs:       raw.Logs,
	}, nil
}

// submitBuild starts a build and returns as soon as the server has queued it.
func submitBuild(ctx context.Context, spec TemplateSpec, opts []SandboxOption) (*BuildInfo, error) {
	body, err := buildTemplateRequest(spec)
	if err != nil {
		return nil, err
	}

	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	respBody, err := client.post(ctx, "/templates/build", body)
	if err != nil {
		return nil, err
	}

	return parseBuildInfo(respBody)
}

// BuildTemplate builds a template from spec and waits for the build to finish,
// polling its status. A build usually takes several minutes; bound the wait
// with ctx. To see the build output as it arrives, start the build with
// BuildTemplateBackground and wait with WaitForBuild instead.
//
// On success the returned BuildInfo has Status BuildStatusCompleted, and
// sandboxes are created from the template by its alias:
// WithTemplate(spec.Alias). A build that fails returns a *BuildError carrying
// the build's logs. Status checks that fail temporarily are retried for up to
// two minutes. Once the build was accepted, the last BuildInfo seen is
// returned alongside any error, so a caller whose ctx expired can keep
// following the build with GetBuildStatus or WaitForBuild.
func BuildTemplate(ctx context.Context, spec TemplateSpec, opts ...SandboxOption) (*BuildInfo, error) {
	info, err := submitBuild(ctx, spec, opts)
	if err != nil {
		return nil, err
	}
	return waitForBuild(ctx, info, nil, opts)
}

// WaitForBuild waits for a build started with BuildTemplateBackground to
// finish, passing each new line of build output to onLog (which may be nil).
// It returns like BuildTemplate: the completed build, a *BuildError for a
// failed one, or ctx's error once ctx ends — the build itself keeps running.
func WaitForBuild(ctx context.Context, buildID string, onLog func(line string), opts ...SandboxOption) (*BuildInfo, error) {
	return waitForBuild(ctx, &BuildInfo{BuildID: buildID, Status: BuildStatusBuilding}, onLog, opts)
}

// waitForBuild polls a started build until it completes or fails.
func waitForBuild(ctx context.Context, info *BuildInfo, onLog func(string), opts []SandboxOption) (*BuildInfo, error) {
	var cursor logCursor
	for {
		if onLog != nil {
			for _, line := range cursor.next(info.Logs) {
				onLog(line)
			}
		}
		switch info.Status {
		case BuildStatusCompleted:
			return info, nil
		case BuildStatusFailed:
			return info, newBuildFailedError(info)
		}

		next, err := pollBuild(ctx, info.BuildID, opts)
		if err != nil {
			return info, err
		}
		info = next
	}
}

// pollBuild waits one poll interval and reads the build's status, repeating
// through temporary failures for up to buildPollErrorWindow.
func pollBuild(ctx context.Context, buildID string, opts []SandboxOption) (*BuildInfo, error) {
	var failingSince time.Time
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(buildPollInterval):
		}

		info, err := GetBuildStatus(ctx, buildID, opts...)
		if err == nil {
			return info, nil
		}
		if !isTemporaryPollError(err) {
			return nil, err
		}
		if failingSince.IsZero() {
			failingSince = time.Now()
		}
		if time.Since(failingSince) >= buildPollErrorWindow {
			return nil, err
		}
	}
}

// newBuildFailedError builds the error for a failed build, quoting the end of
// its logs.
func newBuildFailedError(info *BuildInfo) error {
	msg := fmt.Sprintf("template build %s failed", info.BuildID)
	tail := info.Logs
	if len(tail) > buildFailureLogLines {
		tail = tail[len(tail)-buildFailureLogLines:]
	}
	if len(tail) > 0 {
		msg += ":\n" + strings.Join(tail, "\n")
	}
	return &BuildError{
		SandboxError: &SandboxError{Message: msg},
		BuildID:      info.BuildID,
		Logs:         info.Logs,
	}
}

// BuildTemplateBackground starts a template build and returns as soon as the
// server has accepted it, with Status BuildStatusBuilding. Follow the build
// with GetBuildStatus.
func BuildTemplateBackground(ctx context.Context, spec TemplateSpec, opts ...SandboxOption) (*BuildInfo, error) {
	return submitBuild(ctx, spec, opts)
}

// GetBuildStatus returns the current status of a template build, including
// its logs so far.
func GetBuildStatus(ctx context.Context, buildID string, opts ...SandboxOption) (*BuildInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/templates/builds/%s", buildID)
	respBody, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	return parseBuildInfo(respBody)
}

// ListTemplates returns all templates owned by the authenticated user.
func ListTemplates(ctx context.Context, opts ...SandboxOption) ([]TemplateInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	respBody, err := client.get(ctx, "/templates")
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Templates []struct {
			TemplateID string `json:"template_id"`
			Alias      string `json:"alias"`
			CreatedAt  string `json:"created_at"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing templates list: %w", err)
	}
	raw := wrapper.Templates

	templates := make([]TemplateInfo, len(raw))
	for i, r := range raw {
		templates[i] = TemplateInfo{
			TemplateID: r.TemplateID,
			Alias:      r.Alias,
			CreatedAt:  r.CreatedAt,
		}
	}
	return templates, nil
}

// GetTemplate returns information about a specific template.
func GetTemplate(ctx context.Context, templateID string, opts ...SandboxOption) (*TemplateInfo, error) {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/templates/%s", templateID)
	respBody, err := client.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		TemplateID string `json:"template_id"`
		Alias      string `json:"alias"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("parsing template info: %w", err)
	}

	return &TemplateInfo{
		TemplateID: raw.TemplateID,
		Alias:      raw.Alias,
		CreatedAt:  raw.CreatedAt,
	}, nil
}

// DeleteTemplate deletes a template by its ID.
func DeleteTemplate(ctx context.Context, templateID string, opts ...SandboxOption) error {
	o := resolveSandboxOpts(opts)
	cfg := configFromSandboxOpts(o)
	client := newAPIClient(cfg)

	path := fmt.Sprintf("/templates/%s", templateID)
	_, err := client.delete(ctx, path)
	return err
}
