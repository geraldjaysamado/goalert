package slacktmpl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Limits for the named template library. Individual title/body templates use
// the same 16 KiB limit as the shared Slack.TitleTemplate/TextTemplate
// settings. The total library size is a safety bound for the stored config,
// not a small fixed template count.
const (
	MaxLibraryNameLength     = 100
	MaxLibraryTemplateLength = 16 * 1024
	MaxLibrarySize           = 1024 * 1024
)

// LibraryTemplate is one named Slack alert template.
//
// ID is a stable, generated UUID used for assignments; Name is the editable
// display name.
type LibraryTemplate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Revision identifies the current content of a template. Editors send the
// revision they loaded; an update is rejected if it no longer matches, so one
// admin cannot silently overwrite another admin's changes to the same template.
func (t LibraryTemplate) Revision() string {
	h := sha256.New()
	for _, part := range []string{t.ID, t.Name, t.Title, t.Text} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ErrRevisionConflict means the template changed after the editor loaded it.
var ErrRevisionConflict = &TemplateError{Field: "expectedRevision", Msg: "This template was changed by someone else after you opened it. Your edits were not saved and their changes were kept. Load the latest version, then re-apply your edits."}

// TemplateUpdate is a partial update of one template.
type TemplateUpdate struct {
	ID                string
	ExpectedRevision  string
	Name, Title, Text *string
}

// ApplyUpdate applies u to the library, rejecting it with ErrRevisionConflict
// if the template's current revision differs from u.ExpectedRevision.
func (l Library) ApplyUpdate(u TemplateUpdate) (Library, error) {
	res := append(Library(nil), l...)
	for i := range res {
		if !strings.EqualFold(res[i].ID, u.ID) {
			continue
		}
		if res[i].Revision() != u.ExpectedRevision {
			return nil, ErrRevisionConflict
		}
		if u.Name != nil {
			res[i].Name = strings.TrimSpace(*u.Name)
		}
		if u.Title != nil {
			res[i].Title = *u.Title
		}
		if u.Text != nil {
			res[i].Text = *u.Text
		}
		return res, nil
	}
	return nil, &TemplateError{Field: "id", Msg: "This template no longer exists; it may have been deleted."}
}

// Library is an ordered list of named templates.
type Library []LibraryTemplate

// Find returns the template with the given ID.
func (l Library) Find(id string) (LibraryTemplate, bool) {
	for _, t := range l {
		if strings.EqualFold(t.ID, id) {
			return t, true
		}
	}
	return LibraryTemplate{}, false
}

// ParseLibrary decodes the stored library value. An empty value is an empty
// library.
func ParseLibrary(raw string) (Library, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var lib Library
	if err := json.Unmarshal([]byte(raw), &lib); err != nil {
		return nil, fmt.Errorf("invalid template library data: %w", err)
	}
	return lib, nil
}

// Encode serializes the library for storage. An empty library is stored as an
// empty string so the default config stays unchanged.
func (l Library) Encode() (string, error) {
	if len(l) == 0 {
		return "", nil
	}
	data, err := json.Marshal(l)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TemplateError is a validation problem with one field of a library
// template. Field is one of "id", "name", "title" or "text".
type TemplateError struct {
	Field string
	Msg   string
}

func (e *TemplateError) Error() string { return e.Msg }

func tmplErr(field, format string, args ...any) error {
	return &TemplateError{Field: field, Msg: fmt.Sprintf(format, args...)}
}

// ValidateTemplate checks a single template's name and contents.
func ValidateTemplate(t LibraryTemplate) error {
	if parsed, err := uuid.Parse(t.ID); err != nil || parsed.String() != t.ID {
		return tmplErr("id", "template %q has an invalid ID", t.Name)
	}
	name := strings.TrimSpace(t.Name)
	if name == "" {
		return tmplErr("name", "template name is required")
	}
	if name != t.Name {
		return tmplErr("name", "template name must not start or end with spaces")
	}
	if utf8.RuneCountInString(name) > MaxLibraryNameLength {
		return tmplErr("name", "template name must be at most %d characters", MaxLibraryNameLength)
	}
	if strings.TrimSpace(t.Title) == "" && strings.TrimSpace(t.Text) == "" {
		return tmplErr("text", "template %q: a title or body template is required", t.Name)
	}
	if len(t.Title) > MaxLibraryTemplateLength {
		return tmplErr("title", "template %q: title must be at most %d bytes", t.Name, MaxLibraryTemplateLength)
	}
	if len(t.Text) > MaxLibraryTemplateLength {
		return tmplErr("text", "template %q: body must be at most %d bytes", t.Name, MaxLibraryTemplateLength)
	}
	if err := Validate("title", t.Title); err != nil {
		return tmplErr("title", "template %q title: %v", t.Name, err)
	}
	if err := Validate("body", t.Text); err != nil {
		return tmplErr("text", "template %q body: %v", t.Name, err)
	}
	return nil
}

// ValidateLibrary checks every template and requires unique IDs and
// (case-insensitive) unique names.
func ValidateLibrary(l Library) error {
	ids := make(map[string]struct{}, len(l))
	names := make(map[string]struct{}, len(l))
	for _, t := range l {
		if err := ValidateTemplate(t); err != nil {
			return err
		}
		if _, ok := ids[t.ID]; ok {
			return tmplErr("id", "duplicate template ID %s", t.ID)
		}
		ids[t.ID] = struct{}{}
		key := strings.ToLower(t.Name)
		if _, ok := names[key]; ok {
			return tmplErr("name", "a template named %q already exists", t.Name)
		}
		names[key] = struct{}{}
	}
	return nil
}

// Assignments maps service IDs to template IDs.
type Assignments map[string]string

// ParseAssignments decodes stored "serviceID=templateID" lines.
func ParseAssignments(lines []string) (Assignments, error) {
	res := make(Assignments, len(lines))
	for _, line := range lines {
		svc, tmpl, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			return nil, fmt.Errorf("invalid assignment %q: expected serviceID=templateID", line)
		}
		svc, tmpl = strings.TrimSpace(svc), strings.TrimSpace(tmpl)
		if _, err := uuid.Parse(svc); err != nil {
			return nil, fmt.Errorf("invalid assignment %q: service ID must be a UUID", line)
		}
		if _, err := uuid.Parse(tmpl); err != nil {
			return nil, fmt.Errorf("invalid assignment %q: template ID must be a UUID", line)
		}
		svc = strings.ToLower(svc)
		if _, ok := res[svc]; ok {
			return nil, fmt.Errorf("service %s has more than one template assignment", svc)
		}
		res[svc] = strings.ToLower(tmpl)
	}
	return res, nil
}

// Lookup returns the template ID assigned to the service, if any.
func (a Assignments) Lookup(serviceID string) (string, bool) {
	id, ok := a[strings.ToLower(serviceID)]
	return id, ok
}

// ServicesUsing returns the sorted service IDs assigned to templateID.
func (a Assignments) ServicesUsing(templateID string) []string {
	var res []string
	for svc, tmpl := range a {
		if strings.EqualFold(tmpl, templateID) {
			res = append(res, svc)
		}
	}
	sort.Strings(res)
	return res
}

// Encode returns the stored, sorted "serviceID=templateID" lines.
func (a Assignments) Encode() []string {
	if len(a) == 0 {
		return nil
	}
	res := make([]string, 0, len(a))
	for svc, tmpl := range a {
		res = append(res, svc+"="+tmpl)
	}
	sort.Strings(res)
	return res
}

// ValidateAssignments checks that every assignment references a template in
// the library. This is what blocks deleting a template that is still in use.
func ValidateAssignments(l Library, a Assignments) error {
	missing := make(map[string][]string)
	for svc, tmpl := range a {
		if _, ok := l.Find(tmpl); !ok {
			missing[tmpl] = append(missing[tmpl], svc)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	tmplIDs := make([]string, 0, len(missing))
	for id := range missing {
		tmplIDs = append(tmplIDs, id)
	}
	sort.Strings(tmplIDs)
	svcs := missing[tmplIDs[0]]
	sort.Strings(svcs)
	return fmt.Errorf("template %s does not exist but is assigned to service(s) %s; reassign those services first", tmplIDs[0], strings.Join(svcs, ", "))
}
