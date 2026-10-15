package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportStandingsAndRoundTallies(t *testing.T) {
	g := NewGame()
	g.AddOrUpdatePlayer("a", "Alice")
	g.AddOrUpdatePlayer("b", "Bob")
	g.AddOrUpdatePlayer("c", "Carol")

	g.StartRound(track("t1", "Imagine", "John Lennon"))
	g.SubmitAnswer("a", "Imagine", "John Lennon")
	g.SubmitAnswer("b", "Imagine", "nope")
	g.SubmitAnswer("c", "nope", "Lennon")

	g.StartRound(track("t2", "Yesterday", "The Beatles"))
	g.SubmitAnswer("b", "Yesterday", "The Beatles")
	g.EndRound()

	rec, live := g.GameRecord("")
	if rec == nil || !live {
		t.Fatalf("expected live game record, got %v live=%v", rec, live)
	}
	v := buildExportView(rec, live)
	if len(v.Rounds) != 2 {
		t.Fatalf("want 2 rounds, got %d", len(v.Rounds))
	}
	if r := v.Rounds[0]; r.SongsRight != 2 || r.ArtistsRight != 2 {
		t.Fatalf("round 1 tallies: songs=%d artists=%d", r.SongsRight, r.ArtistsRight)
	}
	// Bob 3, Alice 2, Carol 1.
	want := []struct {
		name                  string
		rank, score, answered int
	}{{"Bob", 1, 3, 2}, {"Alice", 2, 2, 1}, {"Carol", 3, 1, 1}}
	for i, w := range want {
		s := v.Standings[i]
		if s.Name != w.name || s.Rank != w.rank || s.Score != w.score || s.Answered != w.answered {
			t.Fatalf("standing %d = %+v, want %+v", i, s, w)
		}
	}
}

func TestExportTiedScoresShareRank(t *testing.T) {
	v := buildExportView(&GameRecord{Players: []Player{
		{ID: "a", Name: "A", Score: 4}, {ID: "b", Name: "B", Score: 4}, {ID: "c", Name: "C", Score: 1},
	}}, false)
	got := []int{v.Standings[0].Rank, v.Standings[1].Rank, v.Standings[2].Rank}
	if got[0] != 1 || got[1] != 1 || got[2] != 3 {
		t.Fatalf("ranks = %v, want [1 1 3]", got)
	}
}

func TestExportArchivedGameByID(t *testing.T) {
	g := NewGame()
	g.AddOrUpdatePlayer("a", "Alice")
	g.StartRound(track("t1", "Imagine", "John Lennon"))
	g.SubmitAnswer("a", "Imagine", "John Lennon")
	id := g.GameID
	g.EndGame()

	if rec, _ := g.GameRecord(""); rec != nil {
		t.Fatalf("no live game after EndGame, got %+v", rec)
	}
	rec, live := g.GameRecord(id)
	if rec == nil || live {
		t.Fatalf("archived game %s not found (live=%v)", id, live)
	}
	if v := buildExportView(rec, live); v.Standings[0].Score != 2 || v.EndedAt.IsZero() {
		t.Fatalf("archived export lost final score or end time: %+v", v)
	}
	if rec, _ := g.GameRecord("missing"); rec != nil {
		t.Fatalf("unknown id should not resolve")
	}
}

func TestExportHandlerRendersAndRequiresAdmin(t *testing.T) {
	s := NewServer(ServerConfig{})
	g := s.Game()
	g.AddOrUpdatePlayer("a", "Alice")
	g.StartRound(track("t1", "Imagine", "John Lennon"))
	g.SubmitAnswer("a", "<b>Imagine</b>", "John Lennon")

	req := httptest.NewRequest("GET", "/admin/export", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin got %d, want 403", rec.Code)
	}

	req = httptest.NewRequest("GET", "/admin/export", nil)
	req.AddCookie(&http.Cookie{Name: adminCookie, Value: s.AdminToken()})
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("admin got %d: %s", rec.Code, body)
	}
	for _, want := range []string{"Final standings", "Alice", "Round 1", "Imagine", "John Lennon", "&lt;b&gt;Imagine&lt;/b&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("export page missing %q", want)
		}
	}
}
