package httpapi

import (
	"net/http"
	"testing"
)

type priorityList struct {
	Items []priorityJSON `json:"items"`
}

func priorities(t *testing.T, c *client) map[string]priorityJSON {
	t.Helper()
	var out priorityList
	c.call(http.MethodGet, "/repair-priorities", nil, http.StatusOK, &out)
	m := map[string]priorityJSON{}
	for _, p := range out.Items {
		key := ""
		if p.BuiltinKey != nil {
			key = *p.BuiltinKey
		}
		if p.CustomName != nil {
			key = *p.CustomName
		}
		m[key] = p
	}
	return m
}

func newRepair(t *testing.T, c *client, vehicleID int64, title string, extra map[string]any) repairJSON {
	t.Helper()
	body := map[string]any{"vehicle_id": vehicleID, "title": title}
	for k, v := range extra {
		body[k] = v
	}
	var out repairResp
	c.call(http.MethodPost, "/repairs", body, http.StatusCreated, &out)
	return out.Repair
}

func TestRepairPriorities(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 100000)

	p := priorities(t, c)
	if len(p) != 4 || !p["normal"].IsDefault || p["urgent"].SortOrder != 1 || p["low"].Color != "zinc" {
		t.Fatalf("built-ins %+v", p)
	}

	// New projects get the default unless they choose; null means none.
	def := newRepair(t, c, v.ID, "Default", nil)
	if def.PriorityID == nil || *def.PriorityID != p["normal"].ID {
		t.Fatalf("default priority %v", def.PriorityID)
	}
	none := newRepair(t, c, v.ID, "None", map[string]any{"priority_id": nil})
	if none.PriorityID != nil {
		t.Fatalf("none got %v", *none.PriorityID)
	}
	urgent := newRepair(t, c, v.ID, "Urgent", map[string]any{"priority_id": p["urgent"].ID, "status": "in_progress"})
	low := newRepair(t, c, v.ID, "Low", map[string]any{"priority_id": p["low"].ID})
	if code := c.errorCode(http.MethodPost, "/repairs", map[string]any{"vehicle_id": v.ID, "title": "x", "priority_id": 999}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("unknown priority: %q", code)
	}

	// PATCH: omitted keeps, null clears, a value sets.
	var got repairResp
	c.call(http.MethodPatch, "/repairs/"+itoa(urgent.ID), map[string]any{"title": "Urgent brakes"}, http.StatusOK, &got)
	if got.Repair.PriorityID == nil || *got.Repair.PriorityID != p["urgent"].ID {
		t.Fatalf("priority not kept: %v", got.Repair.PriorityID)
	}
	c.call(http.MethodPatch, "/repairs/"+itoa(def.ID), map[string]any{"priority_id": p["high"].ID}, http.StatusOK, &got)
	if *got.Repair.PriorityID != p["high"].ID {
		t.Fatal("priority not set")
	}

	titles := func(query string) []string {
		var list listResponse[repairSummaryJSON]
		c.call(http.MethodGet, "/repairs"+query, nil, http.StatusOK, &list)
		out := make([]string, 0, len(list.Items))
		for _, r := range list.Items {
			out = append(out, r.Title)
		}
		return out
	}
	// Priority order, projects without one last; status breaks ties.
	if got := titles("?sort=priority"); !equalStrings(got, []string{"Urgent brakes", "Default", "Low", "None"}) {
		t.Fatalf("sort=priority %v", got)
	}
	// The default order is unchanged: status first (in progress before planned).
	if got := titles(""); got[0] != "Urgent brakes" || len(got) != 4 {
		t.Fatalf("default sort %v", got)
	}
	if got := titles("?priority=" + itoa(p["low"].ID) + "&priority=none&sort=priority"); !equalStrings(got, []string{"Low", "None"}) {
		t.Fatalf("filter %v", got)
	}
	c.errorCode(http.MethodGet, "/repairs?sort=random", nil, http.StatusBadRequest)
	c.errorCode(http.MethodGet, "/repairs?priority=abc", nil, http.StatusBadRequest)

	// Rename, recolor, reset a built-in; create a custom one as the new default.
	var one struct {
		Priority priorityJSON `json:"priority"`
	}
	c.call(http.MethodPatch, "/repair-priorities/"+itoa(p["urgent"].ID), map[string]any{"name": "Critical", "color": "pink"}, http.StatusOK, &one)
	if *one.Priority.CustomName != "Critical" || one.Priority.Color != "pink" || one.Priority.ProjectCount != 1 {
		t.Fatalf("renamed %+v", one.Priority)
	}
	c.call(http.MethodPatch, "/repair-priorities/"+itoa(p["urgent"].ID), map[string]any{"name": ""}, http.StatusOK, &one)
	if one.Priority.CustomName != nil {
		t.Fatal("reset kept the custom name")
	}
	c.call(http.MethodPost, "/repair-priorities", map[string]any{"name": "Someday", "color": "violet", "is_default": true}, http.StatusCreated, &one)
	someday := one.Priority
	if someday.SortOrder != 5 || !someday.IsDefault {
		t.Fatalf("created %+v", someday)
	}
	if priorities(t, c)["normal"].IsDefault {
		t.Fatal("two defaults")
	}
	for _, bad := range []map[string]any{{"name": "someday", "color": "red"}, {"name": "X", "color": "#ff0000"}, {"name": "", "color": "red"}, {"color": "red"}} {
		if code := c.errorCode(http.MethodPost, "/repair-priorities", bad, http.StatusUnprocessableEntity); code != "validation.failed" {
			t.Fatalf("%v: %q", bad, code)
		}
	}

	// Reorder needs every id once.
	ids := []int64{someday.ID, p["urgent"].ID, p["high"].ID, p["normal"].ID, p["low"].ID}
	c.call(http.MethodPut, "/repair-priorities/order", map[string]any{"ids": ids}, http.StatusOK, nil)
	if got := priorities(t, c)["Someday"].SortOrder; got != 1 {
		t.Fatalf("someday order %d", got)
	}
	if code := c.errorCode(http.MethodPut, "/repair-priorities/order", map[string]any{"ids": ids[:4]}, http.StatusUnprocessableEntity); code != "priority.order_mismatch" {
		t.Fatalf("partial order: %q", code)
	}

	// Delete with move, and without (projects lose their priority).
	c.call(http.MethodDelete, "/repair-priorities/"+itoa(p["low"].ID)+"?move_to="+itoa(someday.ID), nil, http.StatusNoContent, nil)
	c.call(http.MethodGet, "/repairs/"+itoa(low.ID), nil, http.StatusOK, &got)
	if *got.Repair.PriorityID != someday.ID {
		t.Fatal("not moved")
	}
	c.call(http.MethodDelete, "/repair-priorities/"+itoa(someday.ID), nil, http.StatusNoContent, nil)
	c.call(http.MethodGet, "/repairs/"+itoa(low.ID), nil, http.StatusOK, &got)
	if got.Repair.PriorityID != nil {
		t.Fatal("deleted priority still set")
	}
	// The default was deleted: new projects get none.
	if r := newRepair(t, c, v.ID, "After", nil); r.PriorityID != nil {
		t.Fatal("deleted default still applied")
	}
	if code := c.errorCode(http.MethodDelete, "/repair-priorities/"+itoa(p["high"].ID)+"?move_to="+itoa(p["high"].ID), nil, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("move to itself: %q", code)
	}
}

func TestPriorityDisplayPreference(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	var out struct {
		User userJSON `json:"user"`
	}
	c.call(http.MethodPatch, "/me/preferences", map[string]any{"priority_display": "color"}, http.StatusOK, &out)
	if out.User.PriorityDisplay != "color" {
		t.Fatalf("user %+v", out.User)
	}
	if code := c.errorCode(http.MethodPatch, "/me/preferences", map[string]any{"priority_display": "emoji"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
}

func equalStrings(a, b []string) bool {
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
