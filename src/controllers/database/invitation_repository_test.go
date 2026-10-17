package database

import (
	"testing"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/testutil"
)

func TestInvitationRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetInvitationByToken not found", func(t *testing.T) {
		repo := NewInvitationRepository(db)

		_, err := repo.GetInvitationByToken("nonexistent-token")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if err != errors.ErrInvitationNotFound {
			t.Errorf("expected ErrInvitationNotFound, got %v", err)
		}
	})

	t.Run("CancelInvitation not found", func(t *testing.T) {
		repo := NewInvitationRepository(db)

		err := repo.CancelInvitation(9999, 1)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if err != errors.ErrInvitationNotFound {
			t.Errorf("expected ErrInvitationNotFound, got %v", err)
		}
	})
}
