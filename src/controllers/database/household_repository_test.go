package database

import (
	"testing"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func TestHouseholdRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetHouseholdByID", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		household := testutil.CreateTestHousehold(db, 0)

		foundHousehold, err := repo.GetHouseholdByID(household.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if foundHousehold.ID != household.ID {
			t.Errorf("household.ID = %v, want %v", foundHousehold.ID, household.ID)
		}
	})

	t.Run("GetHouseholdByID not found", func(t *testing.T) {
		repo := NewHouseholdRepository(db)

		_, err := repo.GetHouseholdByID(9999)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("GetHouseholdMemberCount", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		testutil.CreateTestUser(db, household.ID)
		testutil.CreateTestUser(db, household.ID)

		count, err := repo.GetHouseholdMemberCount(household.ID)
		if err != nil {
			t.Fatalf("GetHouseholdMemberCount() error = %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("GetHouseholdMembers", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		other := testutil.CreateTestHousehold(db, 0)
		testutil.CreateTestUser(db, household.ID)
		testutil.CreateTestUser(db, other.ID)

		members, err := repo.GetHouseholdMembers(household.ID)
		if err != nil {
			t.Fatalf("GetHouseholdMembers() error = %v", err)
		}
		if len(members) != 1 {
			t.Errorf("members = %d, want 1", len(members))
		}
	})

	t.Run("LeaveHousehold creates solo household and moves products", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID, user.ID)

		if err := repo.LeaveHousehold(user.ID); err != nil {
			t.Fatalf("LeaveHousehold() error = %v", err)
		}

		var moved dbModel.Product
		if err := db.First(&moved, product.ID).Error; err != nil {
			t.Fatalf("failed to reload product: %v", err)
		}
		if moved.HouseholdID == household.ID {
			t.Errorf("product still in old household %d", household.ID)
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if reloaded.HouseholdID == household.ID {
			t.Errorf("user still in old household %d", household.ID)
		}
	})

	t.Run("LeaveHousehold with unknown user errors", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		if err := repo.LeaveHousehold(9999); err == nil {
			t.Error("LeaveHousehold(9999) error = nil, want error")
		}
	})

	t.Run("CreateAndSwitchHousehold seeds default locations", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		if err := repo.CreateAndSwitchHousehold(user.ID, "My New Home"); err != nil {
			t.Fatalf("CreateAndSwitchHousehold() error = %v", err)
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if reloaded.HouseholdID == household.ID {
			t.Errorf("user still in old household %d", household.ID)
		}

		var locations []dbModel.StorageLocation
		if err := db.Where("household_id = ?", reloaded.HouseholdID).Find(&locations).Error; err != nil {
			t.Fatalf("failed to load locations: %v", err)
		}
		if len(locations) != 3 {
			t.Errorf("locations = %d, want 3 defaults", len(locations))
		}
	})

	t.Run("CreateAndSwitchHousehold with unknown user errors", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		if err := repo.CreateAndSwitchHousehold(9999, "Nope"); err == nil {
			t.Error("CreateAndSwitchHousehold(9999) error = nil, want error")
		}
	})
}

func TestHouseholdRepository_Applications(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewHouseholdRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	admin := testutil.CreateTestUser(db, household.ID)
	household.AdminID = admin.ID
	if err := db.Save(household).Error; err != nil {
		t.Fatalf("failed to save household: %v", err)
	}
	applicant := testutil.CreateTestUser(db, 0)

	t.Run("ApplyForHousehold creates pending application", func(t *testing.T) {
		if err := repo.ApplyForHousehold(applicant.ID, household.ID); err != nil {
			t.Fatalf("ApplyForHousehold() error = %v", err)
		}

		pending, err := repo.GetPendingApplicationsForAdmin(admin.ID)
		if err != nil {
			t.Fatalf("GetPendingApplicationsForAdmin() error = %v", err)
		}
		if len(pending) != 1 || pending[0].ApplicantID != applicant.ID {
			t.Errorf("pending = %+v, want applicant %d", pending, applicant.ID)
		}
	})

	t.Run("ApplyForHousehold twice errors with already pending", func(t *testing.T) {
		if err := repo.ApplyForHousehold(applicant.ID, household.ID); err != errors.ErrApplicationAlreadyPending {
			t.Errorf("error = %v, want %v", err, errors.ErrApplicationAlreadyPending)
		}
	})

	t.Run("ApplyForHousehold for unknown household errors", func(t *testing.T) {
		if err := repo.ApplyForHousehold(applicant.ID, 9999); err != errors.ErrHouseholdNotFound {
			t.Errorf("error = %v, want %v", err, errors.ErrHouseholdNotFound)
		}
	})

	t.Run("GetPendingApplicationsForAdmin rejects non-admin", func(t *testing.T) {
		// A user who belongs to a household they do not admin.
		otherHousehold := testutil.CreateTestHousehold(db, 0)
		nonAdmin := testutil.CreateTestUser(db, otherHousehold.ID)
		if _, err := repo.GetPendingApplicationsForAdmin(nonAdmin.ID); err != errors.ErrNotHouseholdAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrNotHouseholdAdmin)
		}
	})

	t.Run("GetPendingApplicationsForApplicant lists own applications", func(t *testing.T) {
		pending, err := repo.GetPendingApplicationsForApplicant(applicant.ID)
		if err != nil {
			t.Fatalf("GetPendingApplicationsForApplicant() error = %v", err)
		}
		if len(pending) != 1 {
			t.Errorf("pending = %d, want 1", len(pending))
		}
	})

	t.Run("CancelApplication removes own pending application", func(t *testing.T) {
		second := testutil.CreateTestUser(db, 0)
		if err := repo.ApplyForHousehold(second.ID, household.ID); err != nil {
			t.Fatalf("ApplyForHousehold() error = %v", err)
		}
		pending, _ := repo.GetPendingApplicationsForApplicant(second.ID)
		if len(pending) != 1 {
			t.Fatalf("pending = %d, want 1", len(pending))
		}

		if err := repo.CancelApplication(pending[0].ID, second.ID); err != nil {
			t.Fatalf("CancelApplication() error = %v", err)
		}
		if err := repo.CancelApplication(pending[0].ID, second.ID); err != errors.ErrApplicationNotFound {
			t.Errorf("second cancel error = %v, want %v", err, errors.ErrApplicationNotFound)
		}
	})

	t.Run("CancelApplication by non-applicant errors", func(t *testing.T) {
		pending, _ := repo.GetPendingApplicationsForApplicant(applicant.ID)
		if len(pending) != 1 {
			t.Fatalf("pending = %d, want 1", len(pending))
		}
		other := testutil.CreateTestUser(db, 0)
		if err := repo.CancelApplication(pending[0].ID, other.ID); err != errors.ErrNotApplicationApplicant {
			t.Errorf("error = %v, want %v", err, errors.ErrNotApplicationApplicant)
		}
	})

	t.Run("ApproveApplication moves applicant into household", func(t *testing.T) {
		pending, _ := repo.GetPendingApplicationsForApplicant(applicant.ID)
		if len(pending) != 1 {
			t.Fatalf("pending = %d, want 1", len(pending))
		}

		if err := repo.ApproveApplication(pending[0].ID, admin.ID); err != nil {
			t.Fatalf("ApproveApplication() error = %v", err)
		}

		var moved authentication.User
		if err := db.First(&moved, applicant.ID).Error; err != nil {
			t.Fatalf("failed to reload applicant: %v", err)
		}
		if moved.HouseholdID != household.ID {
			t.Errorf("applicant household = %d, want %d", moved.HouseholdID, household.ID)
		}
		if moved.Role != authentication.RoleMember {
			t.Errorf("applicant role = %q, want member", moved.Role)
		}
	})

	t.Run("ApproveApplication by non-admin errors", func(t *testing.T) {
		third := testutil.CreateTestUser(db, 0)
		if err := repo.ApplyForHousehold(third.ID, household.ID); err != nil {
			t.Fatalf("ApplyForHousehold() error = %v", err)
		}
		pending, _ := repo.GetPendingApplicationsForApplicant(third.ID)

		if err := repo.ApproveApplication(pending[0].ID, applicant.ID); err != errors.ErrNotHouseholdAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrNotHouseholdAdmin)
		}
	})

	t.Run("RejectApplication marks application rejected", func(t *testing.T) {
		fourth := testutil.CreateTestUser(db, 0)
		if err := repo.ApplyForHousehold(fourth.ID, household.ID); err != nil {
			t.Fatalf("ApplyForHousehold() error = %v", err)
		}
		pending, _ := repo.GetPendingApplicationsForApplicant(fourth.ID)

		if err := repo.RejectApplication(pending[0].ID, admin.ID); err != nil {
			t.Fatalf("RejectApplication() error = %v", err)
		}

		var application dbModel.HouseholdApplication
		if err := db.First(&application, pending[0].ID).Error; err != nil {
			t.Fatalf("failed to reload application: %v", err)
		}
		if application.Status != dbModel.ApplicationStatusRejected {
			t.Errorf("status = %q, want rejected", application.Status)
		}
	})

	t.Run("ApproveApplication with unknown application errors", func(t *testing.T) {
		if err := repo.ApproveApplication(9999, admin.ID); err != errors.ErrApplicationNotFound {
			t.Errorf("error = %v, want %v", err, errors.ErrApplicationNotFound)
		}
	})
}

func TestHouseholdRepository_NameSettingsAndMembers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewHouseholdRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	admin := testutil.CreateTestUser(db, household.ID)
	household.AdminID = admin.ID
	if err := db.Save(household).Error; err != nil {
		t.Fatalf("failed to save household: %v", err)
	}
	member := testutil.CreateTestUser(db, household.ID)

	t.Run("UpdateHouseholdName as admin", func(t *testing.T) {
		if err := repo.UpdateHouseholdName(household.ID, admin.ID, "Renamed"); err != nil {
			t.Fatalf("UpdateHouseholdName() error = %v", err)
		}
		var updated dbModel.Household
		if err := db.First(&updated, household.ID).Error; err != nil {
			t.Fatalf("failed to reload household: %v", err)
		}
		if updated.Name != "Renamed" {
			t.Errorf("name = %q, want Renamed", updated.Name)
		}
	})

	t.Run("UpdateHouseholdName by non-admin errors", func(t *testing.T) {
		if err := repo.UpdateHouseholdName(household.ID, member.ID, "Nope"); err != errors.ErrNotHouseholdAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrNotHouseholdAdmin)
		}
	})

	t.Run("UpdateHouseholdName unknown household errors", func(t *testing.T) {
		if err := repo.UpdateHouseholdName(9999, admin.ID, "Nope"); err != errors.ErrHouseholdNotFound {
			t.Errorf("error = %v, want %v", err, errors.ErrHouseholdNotFound)
		}
	})

	t.Run("UpdateHouseholdSettings as admin", func(t *testing.T) {
		goalCount := 5
		goalPercent := 12.5
		if err := repo.UpdateHouseholdSettings(household.ID, admin.ID, "count", &goalCount, &goalPercent); err != nil {
			t.Fatalf("UpdateHouseholdSettings() error = %v", err)
		}
		var updated dbModel.Household
		if err := db.First(&updated, household.ID).Error; err != nil {
			t.Fatalf("failed to reload household: %v", err)
		}
		if updated.MonthlyWasteGoalType != "count" {
			t.Errorf("goal type = %q, want count", updated.MonthlyWasteGoalType)
		}
		if updated.MonthlyWasteGoalCount == nil || *updated.MonthlyWasteGoalCount != 5 {
			t.Errorf("goal count = %v, want 5", updated.MonthlyWasteGoalCount)
		}
		if updated.MonthlyWasteGoalPercent == nil || *updated.MonthlyWasteGoalPercent != 12.5 {
			t.Errorf("goal percent = %v, want 12.5", updated.MonthlyWasteGoalPercent)
		}
	})

	t.Run("UpdateHouseholdSettings by non-admin errors", func(t *testing.T) {
		if err := repo.UpdateHouseholdSettings(household.ID, member.ID, "count", nil, nil); err != errors.ErrNotHouseholdAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrNotHouseholdAdmin)
		}
	})

	t.Run("RemoveMemberFromHousehold moves member to solo household", func(t *testing.T) {
		if err := repo.RemoveMemberFromHousehold(member.ID, admin.ID); err != nil {
			t.Fatalf("RemoveMemberFromHousehold() error = %v", err)
		}
		var moved authentication.User
		if err := db.First(&moved, member.ID).Error; err != nil {
			t.Fatalf("failed to reload member: %v", err)
		}
		if moved.HouseholdID == household.ID {
			t.Errorf("member still in household %d", household.ID)
		}
	})

	t.Run("RemoveMemberFromHousehold by non-admin errors", func(t *testing.T) {
		nonAdmin := testutil.CreateTestUser(db, household.ID)
		outsider := testutil.CreateTestUser(db, 0)
		if err := repo.RemoveMemberFromHousehold(outsider.ID, nonAdmin.ID); err != errors.ErrNotHouseholdAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrNotHouseholdAdmin)
		}
	})

	t.Run("RemoveMemberFromHousehold of admin errors", func(t *testing.T) {
		if err := repo.RemoveMemberFromHousehold(admin.ID, admin.ID); err != errors.ErrCannotRemoveAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrCannotRemoveAdmin)
		}
	})

	t.Run("SetHouseholdMemberRole as admin", func(t *testing.T) {
		newMember := testutil.CreateTestUser(db, household.ID)
		if err := repo.SetHouseholdMemberRole(newMember.ID, admin.ID, authentication.RoleAdmin); err != nil {
			t.Fatalf("SetHouseholdMemberRole() error = %v", err)
		}
		var updated authentication.User
		if err := db.First(&updated, newMember.ID).Error; err != nil {
			t.Fatalf("failed to reload member: %v", err)
		}
		if updated.Role != authentication.RoleAdmin {
			t.Errorf("role = %q, want admin", updated.Role)
		}
	})

	t.Run("SetHouseholdMemberRole demoting last admin errors", func(t *testing.T) {
		if err := repo.SetHouseholdMemberRole(admin.ID, admin.ID, authentication.RoleMember); err != errors.ErrInsufficientRole {
			t.Errorf("error = %v, want %v", err, errors.ErrInsufficientRole)
		}
	})

	t.Run("SetHouseholdMemberRole by non-admin errors", func(t *testing.T) {
		nonAdmin := testutil.CreateTestUser(db, household.ID)
		outsider := testutil.CreateTestUser(db, 0)
		if err := repo.SetHouseholdMemberRole(outsider.ID, nonAdmin.ID, authentication.RoleAdmin); err != errors.ErrNotHouseholdAdmin {
			t.Errorf("error = %v, want %v", err, errors.ErrNotHouseholdAdmin)
		}
	})

	t.Run("GetPublicHouseholds excludes caller household and counts members", func(t *testing.T) {
		results, err := repo.GetPublicHouseholds(household.ID)
		if err != nil {
			t.Fatalf("GetPublicHouseholds() error = %v", err)
		}
		for _, h := range results {
			if h.ID == household.ID {
				t.Errorf("results include excluded household %d", household.ID)
			}
		}
	})
}
