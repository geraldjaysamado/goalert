package slacktmpl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idA  = "6f1c2c62-55d4-4f43-8a1e-3a4f7f7c9a01"
	idB  = "6f1c2c62-55d4-4f43-8a1e-3a4f7f7c9a02"
	svc1 = "0d3b0d4e-0000-4000-8000-000000000001"
	svc2 = "0d3b0d4e-0000-4000-8000-000000000002"
)

func TestLibrary_RoundTripAndValidate(t *testing.T) {
	lib := Library{
		{ID: idA, Name: "Infrastructure — Clear", Title: "{{ .Summary }}", Text: "{{ .Details }}"},
		{ID: idB, Name: "Apps", Title: "{{ .Summary }}"},
	}
	require.NoError(t, ValidateLibrary(lib))

	raw, err := lib.Encode()
	require.NoError(t, err)
	parsed, err := ParseLibrary(raw)
	require.NoError(t, err)
	assert.Equal(t, lib, parsed)

	empty, err := Library(nil).Encode()
	require.NoError(t, err)
	assert.Empty(t, empty, "empty library must keep the default config unchanged")

	got, ok := parsed.Find(idB)
	assert.True(t, ok)
	assert.Equal(t, "Apps", got.Name)
}

func TestLibrary_ValidationErrors(t *testing.T) {
	cases := map[string]struct {
		lib   Library
		field string
	}{
		"bad id":         {Library{{ID: "x", Name: "A", Title: "a"}}, "id"},
		"uppercase id":   {Library{{ID: "6F1C2C62-55D4-4F43-8A1E-3A4F7F7C9A01", Name: "A", Title: "a"}}, "id"},
		"blank name":     {Library{{ID: idA, Name: " ", Title: "a"}}, "name"},
		"empty":          {Library{{ID: idA, Name: "A"}}, "text"},
		"bad title":      {Library{{ID: idA, Name: "A", Title: "{{"}}, "title"},
		"unknown func":   {Library{{ID: idA, Name: "A", Text: "{{ nope }}"}}, "text"},
		"duplicate name": {Library{{ID: idA, Name: "Infra", Title: "a"}, {ID: idB, Name: "infra", Title: "b"}}, "name"},
		"duplicate id":   {Library{{ID: idA, Name: "A", Title: "a"}, {ID: idA, Name: "B", Title: "b"}}, "id"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateLibrary(c.lib)
			require.Error(t, err)
			var tErr *TemplateError
			require.ErrorAs(t, err, &tErr)
			assert.Equal(t, c.field, tErr.Field)
		})
	}
}

func TestAssignments(t *testing.T) {
	a, err := ParseAssignments([]string{svc1 + "=" + idA, " " + svc2 + " = " + idA + " "})
	require.NoError(t, err)
	assert.Equal(t, []string{svc1, svc2}, a.ServicesUsing(idA))
	id, ok := a.Lookup(svc1)
	assert.True(t, ok)
	assert.Equal(t, idA, id)

	// Changing one service leaves the other untouched.
	a[svc1] = idB
	enc := a.Encode()
	assert.Equal(t, []string{svc1 + "=" + idB, svc2 + "=" + idA}, enc)

	lib := Library{{ID: idA, Name: "A", Title: "a"}}
	assert.ErrorContains(t, ValidateAssignments(lib, a), svc1, "assignment to a missing template must be rejected")
	lib = append(lib, LibraryTemplate{ID: idB, Name: "B", Title: "b"})
	assert.NoError(t, ValidateAssignments(lib, a))

	_, err = ParseAssignments([]string{svc1 + "=" + idA, svc1 + "=" + idB})
	assert.Error(t, err, "duplicate service")
	_, err = ParseAssignments([]string{"Slack channel=" + idA})
	assert.Error(t, err, "service IDs must be UUIDs, not names")
	_, err = ParseAssignments([]string{svc1})
	assert.Error(t, err)
}

func TestHelpers(t *testing.T) {
	data := Data{AlertmanagerData: AlertmanagerData{Alerts: Alerts{{}, {}, {}}}}
	out, err := Render("t", `{{ coalesce "" " " "b" "c" }}|{{ len (firstN 2 .Alerts) }}|{{ len (firstN 9 .Alerts) }}|{{ sub 5 2 }}|{{ add 1 2 }}|{{ slackEscape "a<b>&c" }}`, data)
	require.NoError(t, err)
	assert.Equal(t, "b|2|3|3|3|a&lt;b&gt;&amp;c", out)
}

func TestExamplesParse(t *testing.T) {
	for _, e := range Examples() {
		assert.NoError(t, Validate("title", e.Title), e.Key)
		assert.NoError(t, Validate("text", e.Text), e.Key)
		assert.LessOrEqual(t, len(e.Text), MaxLibraryTemplateLength)
	}
}

func TestLibrary_ApplyUpdate_StaleEditors(t *testing.T) {
	str := func(s string) *string { return &s }
	lib := Library{
		{ID: idA, Name: "Infra", Title: "{{ .Summary }}", Text: "v1"},
		{ID: idB, Name: "Apps", Title: "{{ .Summary }}"},
	}
	opened := lib[0].Revision() // both admins open the editor now

	// Admin 1 saves first.
	lib, err := lib.ApplyUpdate(TemplateUpdate{ID: idA, ExpectedRevision: opened, Name: str("Infra"), Title: str("{{ .Summary }}"), Text: str("admin 1")})
	require.NoError(t, err)
	assert.NotEqual(t, opened, lib[0].Revision())

	// Admin 2 saves with the stale revision: rejected, admin 1's text kept.
	_, err = lib.ApplyUpdate(TemplateUpdate{ID: idA, ExpectedRevision: opened, Name: str("Infra"), Title: str("{{ .Summary }}"), Text: str("admin 2")})
	require.ErrorIs(t, err, ErrRevisionConflict)
	assert.Equal(t, "admin 1", lib[0].Text)

	// After reloading, admin 2 can save.
	lib, err = lib.ApplyUpdate(TemplateUpdate{ID: idA, ExpectedRevision: lib[0].Revision(), Text: str("admin 2")})
	require.NoError(t, err)
	assert.Equal(t, "admin 2", lib[0].Text)
	assert.Equal(t, "Apps", lib[1].Name, "other templates untouched")

	_, err = lib.ApplyUpdate(TemplateUpdate{ID: "6f1c2c62-55d4-4f43-8a1e-3a4f7f7c9aff", ExpectedRevision: "x"})
	var tErr *TemplateError
	require.ErrorAs(t, err, &tErr)
	assert.Equal(t, "id", tErr.Field)
}
