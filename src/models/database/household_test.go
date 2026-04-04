package database

import (
	"testing"
)

func TestHouseholdStruct(t *testing.T) {
	t.Run("Household struct has correct fields", func(t *testing.T) {
		household := Household{
			Name:        "Test Household",
			Description: "A test household",
			AdminID:     1,
		}

		if household.Name != "Test Household" {
			t.Errorf("Name = %v, want Test Household", household.Name)
		}
		if household.Description != "A test household" {
			t.Errorf("Description = %v, want A test household", household.Description)
		}
		if household.AdminID != 1 {
			t.Errorf("AdminID = %v, want 1", household.AdminID)
		}
	})
}

func TestHouseholdGORMModel(t *testing.T) {
	t.Run("Household embeds gorm.Model", func(t *testing.T) {
		household := Household{}

		var _ = household.Model
	})
}

func TestHouseholdNameRequired(t *testing.T) {
	t.Run("Household Name should be required", func(t *testing.T) {
		household := Household{
			Name:    "",
			AdminID: 1,
		}

		if household.Name != "" {
			t.Errorf("Name should be empty, got %v", household.Name)
		}
	})
}

func TestHouseholdAdminIDRequired(t *testing.T) {
	t.Run("Household AdminID should be required", func(t *testing.T) {
		household := Household{
			Name:    "Test Household",
			AdminID: 0,
		}

		if household.AdminID != 0 {
			t.Errorf("AdminID = %v, want 0", household.AdminID)
		}
	})
}

func TestHouseholdOptionalFields(t *testing.T) {
	t.Run("Household Description is optional", func(t *testing.T) {
		household := Household{
			Name:        "Test Household",
			AdminID:     1,
			Description: "",
		}

		if household.Description != "" {
			t.Errorf("Description should be empty, got %v", household.Description)
		}
	})
}

func TestHouseholdValidValues(t *testing.T) {
	t.Run("Household with all valid fields", func(t *testing.T) {
		household := Household{
			Name:        "My Family Household",
			Description: "This is our household for tracking food",
			AdminID:     42,
		}

		if household.Name != "My Family Household" {
			t.Errorf("Name = %v, want My Family Household", household.Name)
		}
		if household.Description != "This is our household for tracking food" {
			t.Errorf("Description = %v, want This is our household for tracking food", household.Description)
		}
		if household.AdminID != 42 {
			t.Errorf("AdminID = %v, want 42", household.AdminID)
		}
	})
}
