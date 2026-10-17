package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSidebarRetainsNamedDestinationsAndAccessibleCollapseControl(t *testing.T) {
	var output bytes.Buffer
	if err := Layout("Dashboard", "dashboard", "Example workspace").Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	markup := output.String()
	for _, required := range []string{
		`id="sidebar-collapse"`, `aria-controls="sidebar-navigation"`, `aria-expanded="true"`,
		`id="sidebar-navigation"`, `src="/static/sidebar.js"`,
		`aria-label="Vue d’ensemble"`, `aria-label="Factures"`, `aria-label="Stock"`,
		`aria-label="Comptabilité"`, `aria-label="Trésorerie"`,
		`href="/app/invoices/new"`, `id="nav-toggle"`,
	} {
		if !strings.Contains(markup, required) {
			t.Errorf("sidebar missing %s", required)
		}
	}
}
