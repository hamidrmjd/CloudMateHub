package recordings

import (
	"context"
	"errors"
	"os"

	"github.com/shellhub-io/shellhub/pkg/api/scope"
	"github.com/shellhub-io/shellhub/pkg/models"
	"github.com/shellhub-io/shellhub/server/api/store"
)

func (m *Manager) Delete(ctx context.Context, s *models.Session) error {
	m.work.Lock()
	defer m.work.Unlock()
	// Delete only objects referenced by this exact session's owned manifests.
	for _, seat := range s.Events.Seats {
		a, c, err := m.archived(s.UID, seat)
		if err == nil {
			if err = m.remove(ctx, c, a.Key); err != nil {
				return err
			}
			if err = os.Remove(m.marker(s.UID, seat)); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, seat := range s.Events.Seats {
		if err := m.db.SessionEventsDelete(ctx, models.UID(s.UID), seat, models.SessionEventTypePtyOutput); err != nil {
			return err
		}
	}
	// Keep the session and its non-output audit events intact.
	s.Recorded = false
	updater, ok := m.db.(interface {
		SessionUpdate(context.Context, scope.Scope, *models.Session) error
	})
	if !ok {
		return errors.New("session updater unavailable")
	}
	sc, err := scope.NewBounded(s.TenantID)
	if err != nil {
		return err
	}
	return updater.SessionUpdate(ctx, sc, s)
}
func (m *Manager) DeleteRecordings(ctx context.Context, uids []string) ([]string, error) {
	removed := make([]string, 0, len(uids))
	for _, uid := range uids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s, err := m.db.SessionResolve(ctx, scope.NewUnbounded("remove only expired sessions selected by the recording retention job"), store.SessionUIDResolver, uid)
		if errors.Is(err, store.ErrNoDocuments) {
			removed = append(removed, uid)
			continue
		}
		if err != nil || s.Active {
			continue
		}
		if err = m.Delete(ctx, s); err == nil {
			removed = append(removed, uid)
		}
	}
	return removed, nil
}
