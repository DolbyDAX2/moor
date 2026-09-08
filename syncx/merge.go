package syncx

import (
	"fmt"
	"time"

	"moor/model"
)

// MergeServers, yerel ve uzak listeyi akıllıca birleştirir:
//   - id'si her iki tarafta da varsa: UpdatedAt'ı yenisi kazanır (berabere → yerel)
//   - yalnızca yereldeyse: korunur (yerelde yeni eklenen/değiştirilen)
//   - yalnızca uzaktaysa: eklenir (başka cihazdan gelen)
//
// Sıra kararlıdır: önce yerel liste sırası, sonra uzakta olanlar uzak sırasıyla.
func MergeServers(local, remote []*model.Server) []*model.Server {
	remoteByID := make(map[string]*model.Server, len(remote))
	for _, r := range remote {
		if r.ID != "" {
			remoteByID[r.ID] = r
		}
	}

	out := make([]*model.Server, 0, len(local)+len(remote))
	seen := make(map[string]bool, len(local))
	for _, l := range local {
		seen[l.ID] = true
		r, exists := remoteByID[l.ID]
		if !exists {
			out = append(out, l)
			continue
		}
		var winner, loser *model.Server
		if r.UpdatedAt.After(l.UpdatedAt) {
			winner, loser = r, l
		} else {
			winner, loser = l, r
		}
		inheritKeyMaterial(winner, loser)
		out = append(out, winner)
	}
	for _, r := range remote {
		if !seen[r.ID] {
			seen[r.ID] = true
			out = append(out, r)
		}
	}
	return out
}

// SyncSummary bir senkronizasyon turunun özeti (CLI/GUI çıktısı için).
type SyncSummary struct {
	Version     int64     `json:"version"`
	LocalOnly   int       `json:"localOnly"`   // yerelde var, uzakta yok
	RemoteOnly  int       `json:"remoteOnly"`  // uzakta var, yerelde yok
	ChangedBoth int       `json:"changedBoth"` // ikisinde de var, içeriği farklı
	At          time.Time `json:"at"`
}

// SyncStore tam bir senkronizasyon turu yapar:
//  1. sunucudaki durumu çeker,
//  2. sürüm değişmediyse yerel listeyi doğrudan push'lar;
//     değiştiyse akıllı merge sonrası push'lar,
//  3. çakışma olursa (409) sunucunun döndürdüğü listeyle bir kez daha merge + push.
//
// Dönen merged listesi istemcinin yeni durumudur (store'a yazılır).
func SyncStore(c *Client, local []*model.Server, lastVersion int64) ([]*model.Server, SyncSummary, error) {
	// Anahtar içeriğini senkron verisine göm (Android tarafı için).
	BackfillKeyContent(local)

	state, err := c.GetState()
	if err != nil {
		return nil, SyncSummary{}, fmt.Errorf("durum alınamadı: %w", err)
	}

	var toPush []*model.Server
	base := lastVersion
	if state.Version == lastVersion {
		toPush = local
	} else {
		toPush = MergeServers(local, state.Servers)
		base = state.Version
	}

	newVersion, err := c.Push(base, toPush)
	if IsConflict(err) {
		ce := err.(*ConflictError)
		toPush = MergeServers(toPush, ce.Servers)
		newVersion, err = c.Push(ce.Version, toPush)
	}
	if err != nil {
		return nil, SyncSummary{}, fmt.Errorf("yazılamadı: %w", err)
	}

	sum := SyncSummary{Version: newVersion, At: time.Now()}
	for _, l := range local {
		r := findServer(state.Servers, l.ID)
		switch {
		case r == nil:
			sum.LocalOnly++
		default:
			if !r.UpdatedAt.Equal(l.UpdatedAt) {
				sum.ChangedBoth++
			}
		}
	}
	for _, r := range state.Servers {
		if findServer(local, r.ID) == nil {
			sum.RemoteOnly++
		}
	}
	return toPush, sum, nil
}

func findServer(list []*model.Server, id string) *model.Server {
	for _, s := range list {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// inheritKeyMaterial, kazanan kaydın anahtar içeriği boşken kaybedenin
// doluysa içeriği (ve parolası) devralmasını sağlar. Anahtar içeriği
// updated_at'ı taşımayan bir "ek" alan olduğu için eski kopyanın
// kazanması onu sonsuza dek silebilirdi; bu kural silişi engeller.
func inheritKeyMaterial(winner, loser *model.Server) {
	if winner.AuthType != model.AuthKey {
		return
	}
	if winner.KeyContent != "" || loser.KeyContent == "" {
		return
	}
	winner.KeyContent = loser.KeyContent
	if winner.KeyPassphrase == "" {
		winner.KeyPassphrase = loser.KeyPassphrase
	}
}
