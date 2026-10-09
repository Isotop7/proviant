package services

import (
	"testing"

	"codeberg.org/isotop7/proviant/errors"
	"gorm.io/gorm"
)

func TestBulkClientMiss(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"record not found", gorm.ErrRecordNotFound, true},
		{"mismatched user", errors.ErrMismatcherUserID, true},
		{"invalid user data", errors.ErrInvalidUserData, true},
		{"not implemented", gorm.ErrNotImplemented, true},
		{"concurrent modification", errors.ErrProductConcurrentModification, true},
		{"internal server error", errors.ErrInternalServer, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bulkClientMiss(tt.err); got != tt.want {
				t.Errorf("bulkClientMiss(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
