package ui

import (
	"context"
	"errors"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazyargo/internal/argocd"
	"lazyargo/internal/config"
)

type fakeClient struct {
	apps []argocd.Application

	syncCalls []syncCall
	syncErr   map[string]error
}

type syncCall struct {
	name   string
	dryRun bool
}

func (f *fakeClient) ListApplications(ctx context.Context) ([]argocd.Application, error) {
	return f.apps, nil
}

func (f *fakeClient) GetApplication(ctx context.Context, name string) (argocd.Application, error) {
	return argocd.Application{Name: name}, nil
}

func (f *fakeClient) SyncApplication(ctx context.Context, name string, dryRun bool) error {
	f.syncCalls = append(f.syncCalls, syncCall{name: name, dryRun: dryRun})
	if f.syncErr == nil {
		return nil
	}
	return f.syncErr[name]
}

func TestModel_applyFilter_driftAndQuery(t *testing.T) {
	tests := []struct {
		name      string
		appsAll   []argocd.Application
		query     string
		driftOnly bool
		wantNames []string
	}{
		{
			name:      "no filters",
			appsAll:   []argocd.Application{{Name: "a", Sync: "Synced"}, {Name: "b", Sync: "OutOfSync"}},
			wantNames: []string{"a", "b"},
		},
		{
			name:      "drift only hides synced",
			appsAll:   []argocd.Application{{Name: "a", Sync: "Synced"}, {Name: "b", Sync: "OutOfSync"}, {Name: "c", Sync: ""}},
			driftOnly: true,
			wantNames: []string{"b", "c"},
		},
		{
			name:    "query filter matches substring case-insensitive",
			appsAll: []argocd.Application{{Name: "frontend", Sync: "Synced"}, {Name: "backend", Sync: "OutOfSync"}},
			query:   "END",
			// Default sort is by name.
			wantNames: []string{"backend", "frontend"},
		},
		{
			name:      "query + drift only",
			appsAll:   []argocd.Application{{Name: "frontend", Sync: "Synced"}, {Name: "backend", Sync: "OutOfSync"}, {Name: "worker", Sync: "OutOfSync"}},
			query:     "end",
			driftOnly: true,
			wantNames: []string{"backend"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(config.Default(), &fakeClient{})
			m.appsAll = tt.appsAll
			m.driftOnly = tt.driftOnly
			m.filterInput.SetValue(tt.query)

			m.applyFilter(false)

			got := make([]string, 0, len(m.apps))
			for _, a := range m.apps {
				got = append(got, a.Name)
			}
			if !reflect.DeepEqual(got, tt.wantNames) {
				t.Fatalf("names mismatch\n got: %v\nwant: %v", got, tt.wantNames)
			}
		})
	}
}

func TestModel_syncBatchCmd_dryRunAndReal(t *testing.T) {
	fc := &fakeClient{syncErr: map[string]error{"b": errors.New("boom")}}
	m := NewModel(config.Default(), fc)
	m.appsAll = []argocd.Application{{Name: "a", Sync: "Synced"}, {Name: "b", Sync: "OutOfSync"}, {Name: "c", Sync: "OutOfSync"}}

	// Press 's' to start the dry-run batch.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected a cmd from SyncBatch key")
	}
	msg := cmd()
	batch, ok := msg.(syncBatchMsg)
	if !ok {
		t.Fatalf("expected syncBatchMsg, got %T", msg)
	}
	if !batch.dryRun {
		t.Fatalf("expected dryRun=true")
	}
	if len(fc.syncCalls) != 2 {
		t.Fatalf("expected 2 sync calls, got %d", len(fc.syncCalls))
	}
	if fc.syncCalls[0].dryRun != true || fc.syncCalls[1].dryRun != true {
		t.Fatalf("expected all calls to be dry-run: %+v", fc.syncCalls)
	}
	if fc.syncCalls[0].name != "b" || fc.syncCalls[1].name != "c" {
		t.Fatalf("unexpected call order: %+v", fc.syncCalls)
	}
	if len(batch.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(batch.results))
	}
	if batch.results[0].name != "b" || batch.results[0].err == nil {
		t.Fatalf("expected first result to include error for b, got %+v", batch.results[0])
	}
	if batch.results[1].name != "c" || batch.results[1].err != nil {
		t.Fatalf("expected second result to be success for c, got %+v", batch.results[1])
	}

	// Now simulate being in the modal after a completed dry-run.
	m.syncModal = true
	m.syncTargets = []string{"b", "c"}

	// 'y' should do nothing until the dry-run has completed.
	m.syncDryRunComplete = false
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil {
		t.Fatalf("expected no cmd when dry-run is incomplete")
	}

	// 'y' triggers real sync when the dry-run is complete.
	m.syncDryRunComplete = true
	fc.syncCalls = nil
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected cmd when confirming sync")
	}
	msg = cmd()
	batch, ok = msg.(syncBatchMsg)
	if !ok {
		t.Fatalf("expected syncBatchMsg, got %T", msg)
	}
	if batch.dryRun {
		t.Fatalf("expected dryRun=false for real sync")
	}
	if len(fc.syncCalls) != 2 {
		t.Fatalf("expected 2 sync calls, got %d", len(fc.syncCalls))
	}
	if fc.syncCalls[0].dryRun || fc.syncCalls[1].dryRun {
		t.Fatalf("expected non-dry-run calls: %+v", fc.syncCalls)
	}
}
