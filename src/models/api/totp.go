package api

// TOTPStatusResponse reports the two-factor state of the authenticated user.
// The household policy is not part of it: the settings page already renders it
// from the household it belongs to.
type TOTPStatusResponse struct {
	Enabled                bool  `json:"enabled"`
	Pending                bool  `json:"pending"`
	RecoveryCodesRemaining int64 `json:"recoveryCodesRemaining"`
}

// TOTPEnrolResponse carries the enrolment QR code as a data URI. The TOTP
// secret itself never appears in a response, a log line or the HTML.
type TOTPEnrolResponse struct {
	QR        string `json:"qr"`
	ExpiresAt string `json:"expiresAt"`
}

// TOTPConfirmResponse carries the recovery codes. They are shown exactly once
// and only their hashes are stored.
type TOTPConfirmResponse struct {
	TwoFactorEnabled bool     `json:"twoFactorEnabled"`
	RecoveryCodes    []string `json:"recoveryCodes"`
}
