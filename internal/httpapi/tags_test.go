package httpapi

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
)

func tags(t *testing.T, c *client) map[string]tagJSON {
	t.Helper()
	var out listResponse[tagJSON]
	c.call(http.MethodGet, "/repair-tags", nil, http.StatusOK, &out)
	m := map[string]tagJSON{}
	for _, tag := range out.Items {
		m[tag.Name] = tag
	}
	return m
}

// TestRepairTags covers project tags (#16): one shared list, matched
// regardless of case, AND filtering through ?tag= and the tags: search
// prefix, and renaming or deleting a tag everywhere.
func TestRepairTags(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 100000)

	bleed := newRepair(t, c, v.ID, "Bleed brakes", map[string]any{"symptom": "Insufficient braking power", "tags": []string{" brake  fluid ", "Brembo", "brembo"}})
	if !slices.Equal(bleed.Tags, []string{"brake fluid", "Brembo"}) {
		t.Fatalf("tags %v", bleed.Tags)
	}
	// An existing tag keeps its spelling, whatever case is typed.
	discs := newRepair(t, c, v.ID, "Front discs", map[string]any{"symptom": "Vibration when braking", "tags": []string{"BREMBO", "Brake discs"}})
	if !slices.Equal(discs.Tags, []string{"Brake discs", "Brembo"}) {
		t.Fatalf("tags %v", discs.Tags)
	}
	plain := newRepair(t, c, v.ID, "Wipers", nil)
	if plain.Tags == nil || len(plain.Tags) != 0 {
		t.Fatalf("no tags: %v", plain.Tags)
	}
	if all := tags(t, c); len(all) != 3 || all["Brembo"].ProjectCount != 2 {
		t.Fatalf("tag list %+v", all)
	}

	titles := func(query url.Values) []string {
		var list listResponse[repairSummaryJSON]
		c.call(http.MethodGet, "/repairs?"+query.Encode(), nil, http.StatusOK, &list)
		out := make([]string, 0, len(list.Items))
		for _, r := range list.Items {
			out = append(out, r.Title)
		}
		slices.Sort(out)
		if int(list.Total) != len(out) {
			t.Fatalf("total %d for %v", list.Total, out)
		}
		return out
	}
	for _, tc := range []struct {
		query url.Values
		want  []string
	}{
		{url.Values{"tag": {"brembo"}}, []string{"Bleed brakes", "Front discs"}},
		{url.Values{"tag": {"Brembo", "brake fluid"}}, []string{"Bleed brakes"}},
		{url.Values{"tag": {"unknown"}}, []string{}},
		{url.Values{"q": {"tags:brembo braking"}}, []string{"Bleed brakes", "Front discs"}},
		{url.Values{"q": {`tags:"Brake discs",brembo braking`}}, []string{"Front discs"}},
		{url.Values{"q": {"tags:brembo insufficient braking power"}}, []string{"Bleed brakes"}},
		{url.Values{"q": {"tags:brembo"}, "tag": {"brake fluid"}}, []string{"Bleed brakes"}},
		// Plain text does not match tags.
		{url.Values{"q": {"brembo"}}, []string{}},
	} {
		if got := titles(tc.query); !slices.Equal(got, tc.want) {
			t.Errorf("%v: %v, want %v", tc.query, got, tc.want)
		}
	}

	// PATCH: omitted keeps the tags, a list replaces them.
	var got repairResp
	c.call(http.MethodPatch, "/repairs/"+itoa(bleed.ID), map[string]any{"title": "Bleed brakes (DOT 4)"}, http.StatusOK, &got)
	if len(got.Repair.Tags) != 2 {
		t.Fatalf("tags lost: %v", got.Repair.Tags)
	}
	c.call(http.MethodPatch, "/repairs/"+itoa(bleed.ID), map[string]any{"tags": []string{"brake fluid"}}, http.StatusOK, &got)
	if !slices.Equal(got.Repair.Tags, []string{"brake fluid"}) {
		t.Fatalf("tags %v", got.Repair.Tags)
	}
	for _, bad := range [][]string{{"a,b"}, {`say "hi"`}, {string(make([]byte, 41))}} {
		if code := c.errorCode(http.MethodPatch, "/repairs/"+itoa(bleed.ID), map[string]any{"tags": bad}, http.StatusUnprocessableEntity); code != "validation.failed" {
			t.Fatalf("%q: %q", bad, code)
		}
	}

	// Rename applies everywhere; another tag's name is a duplicate; a case
	// change of the same tag is fine.
	all := tags(t, c)
	var one struct {
		Tag tagJSON `json:"tag"`
	}
	c.call(http.MethodPatch, "/repair-tags/"+itoa(all["Brembo"].ID), map[string]any{"name": "BREMBO"}, http.StatusOK, &one)
	if one.Tag.Name != "BREMBO" || one.Tag.ProjectCount != 1 {
		t.Fatalf("renamed %+v", one.Tag)
	}
	c.call(http.MethodGet, "/repairs/"+itoa(discs.ID), nil, http.StatusOK, &got)
	if !slices.Equal(got.Repair.Tags, []string{"Brake discs", "BREMBO"}) {
		t.Fatalf("after rename %v", got.Repair.Tags)
	}
	if code := c.errorCode(http.MethodPatch, "/repair-tags/"+itoa(all["Brembo"].ID), map[string]any{"name": "brake FLUID"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("duplicate: %q", code)
	}
	c.errorCode(http.MethodPatch, "/repair-tags/999", map[string]any{"name": "x"}, http.StatusNotFound)

	c.call(http.MethodDelete, "/repair-tags/"+itoa(all["Brembo"].ID), nil, http.StatusNoContent, nil)
	c.call(http.MethodGet, "/repairs/"+itoa(discs.ID), nil, http.StatusOK, &got)
	if !slices.Equal(got.Repair.Tags, []string{"Brake discs"}) {
		t.Fatalf("after delete %v", got.Repair.Tags)
	}
	c.errorCode(http.MethodDelete, "/repair-tags/"+itoa(all["Brembo"].ID), nil, http.StatusNotFound)
}

func TestCardTagLimitPreference(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	var out struct {
		User userJSON `json:"user"`
	}
	c.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &out)
	if out.User.CardTagLimit != 3 {
		t.Fatalf("default %d", out.User.CardTagLimit)
	}
	c.call(http.MethodPatch, "/me/preferences", map[string]any{"card_tag_limit": 0}, http.StatusOK, &out)
	if out.User.CardTagLimit != 0 {
		t.Fatalf("user %+v", out.User)
	}
	if code := c.errorCode(http.MethodPatch, "/me/preferences", map[string]any{"card_tag_limit": 21}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
}
