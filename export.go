package main

import (
	"net/http"
	"sort"
	"time"
)

// ExportStanding is one row of the final standings in a game export.
type ExportStanding struct {
	Rank          int
	Name          string
	Score         int
	SongsRight    int
	ArtistsRight  int
	Answered      int
	AvgResponseMS int64
}

// ExportRound is a completed round plus derived tallies for the export page.
type ExportRound struct {
	*RoundResult
	SongsRight   int
	ArtistsRight int
}

// ExportView is everything the printable game summary needs.
type ExportView struct {
	GameID     string
	StartedAt  time.Time
	EndedAt    time.Time // zero while the game is still in progress
	InProgress bool
	Standings  []ExportStanding
	Rounds     []ExportRound
}

// GameRecord returns a game for export: the live game if id is empty or
// matches the current GameID, otherwise the archived game with that id. The
// live game is returned as a snapshot of its completed rounds; an in-progress
// round is not included.
func (g *Game) GameRecord(id string) (rec *GameRecord, live bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if id == "" || id == g.GameID {
		if g.GameID == "" {
			return nil, false
		}
		players := make([]Player, 0, len(g.Players))
		for _, p := range g.Players {
			players = append(players, *p)
		}
		return &GameRecord{
			ID:        g.GameID,
			StartedAt: g.StartedAt,
			Players:   players,
			Rounds:    append([]*RoundResult{}, g.Rounds...),
		}, true
	}
	for i := len(g.History) - 1; i >= 0; i-- {
		if g.History[i].ID == id {
			return g.History[i], false
		}
	}
	return nil, false
}

// buildExportView derives standings and per-round tallies from a game record.
func buildExportView(rec *GameRecord, live bool) ExportView {
	v := ExportView{
		GameID:     rec.ID,
		StartedAt:  rec.StartedAt,
		EndedAt:    rec.EndedAt,
		InProgress: live,
	}
	type tally struct{ songs, artists, answered int }
	tallies := make(map[string]*tally)
	for _, r := range rec.Rounds {
		er := ExportRound{RoundResult: r}
		for _, a := range r.Answers {
			t := tallies[a.PlayerID]
			if t == nil {
				t = &tally{}
				tallies[a.PlayerID] = t
			}
			t.answered++
			if a.SongCorrect {
				t.songs++
				er.SongsRight++
			}
			if a.ArtistCorrect {
				t.artists++
				er.ArtistsRight++
			}
		}
		v.Rounds = append(v.Rounds, er)
	}

	players := append([]Player{}, rec.Players...)
	sort.Slice(players, func(i, j int) bool {
		if players[i].Score != players[j].Score {
			return players[i].Score > players[j].Score
		}
		return players[i].JoinedAt.Before(players[j].JoinedAt)
	})
	for i, p := range players {
		// Tied scores share a rank (1, 1, 3, …).
		rank := i + 1
		if i > 0 && p.Score == players[i-1].Score {
			rank = v.Standings[i-1].Rank
		}
		s := ExportStanding{
			Rank:          rank,
			Name:          p.Name,
			Score:         p.Score,
			AvgResponseMS: p.AvgResponseMS(),
		}
		if t := tallies[p.ID]; t != nil {
			s.SongsRight, s.ArtistsRight, s.Answered = t.songs, t.artists, t.answered
		}
		v.Standings = append(v.Standings, s)
	}
	return v
}

// handleExport renders a printable summary of a game: the current game by
// default, or an archived one via ?game=<id>.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rec, live := s.game.GameRecord(r.URL.Query().Get("game"))
	if rec == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}
	s.render(w, "export.html", buildExportView(rec, live))
}
