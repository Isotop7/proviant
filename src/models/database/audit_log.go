package database

import (
	"time"

	"gorm.io/gorm"
)

type AuditLog struct {
	gorm.Model
	Timestamp time.Time `gorm:"index, not null" json:"timestamp"`
	UserID    *uint     `gorm:"index" json:"userId"`
	Action    string    `gorm:"index, not null" json:"action"`
	IPAddress string    `gorm:"not null" json:"ipAddress"`
	Details   string    `gorm:"type:text" json:"details"`
	RequestID string    `gorm:"index" json:"requestId"`
}

const (
	AuditActionLoginSuccess         = "login_success"
	AuditActionLoginFailure         = "login_failure"
	AuditActionPasswordChange       = "password_change"
	AuditActionPasswordChangeFailed = "password_change_failed"
	AuditActionMemberAdded          = "member_added"
	AuditActionMemberRemoved        = "member_removed"
	AuditActionMemberLeft           = "member_left"
	AuditActionAdminChanged         = "admin_changed"
	AuditActionAccountDeleted       = "account_deleted"
)
