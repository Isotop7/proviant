package mocks

import (
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
)

// MockProductRepository is a configurable in-memory stub for ProductRepositoryInterface.
type MockProductRepository struct {
	Products      []dbModel.Product
	Product       dbModel.Product
	OFFCache      dbModel.OpenFoodFactsCache
	Household     dbModel.Household
	User          authentication.User
	Users         []authentication.User
	StatsMonthly  []apiModel.StatsMonthlyCount
	ExpiringProds []apiModel.StatsExpiringProduct
	BoolResult    bool
	IntResult     int
	MapResult     map[string]int
	Err           error
}

func (m *MockProductRepository) GetUserProductsBulk(userID uint, limit int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetUserArchivedProductsBulk(userID uint, limit int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetUserProductsBulkByBarcode(userID uint, barcode int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetProductByID(productID, userID uint) (dbModel.Product, error) {
	return m.Product, m.Err
}
func (m *MockProductRepository) GetArchivedProductByID(productID, userID uint) (dbModel.Product, error) {
	return m.Product, m.Err
}
func (m *MockProductRepository) SearchProducts(queryParam database.SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetUserProductsByLocation(userID, locationID uint) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) CreateProduct(userID uint, product *dbModel.Product) error {
	return m.Err
}
func (m *MockProductRepository) UpdateProduct(productID uint, userID uint, product *dbModel.ProductDTOPatch) error {
	return m.Err
}
func (m *MockProductRepository) UpdateProductAmount(productID uint, userID uint, delta int) (bool, error) {
	return m.BoolResult, m.Err
}
func (m *MockProductRepository) DeleteProduct(productID uint, userID uint, archiveOnly bool) error {
	return m.Err
}
func (m *MockProductRepository) BulkDeleteProducts(productIDs []uint, userID uint) []database.BulkOperationError {
	return nil
}
func (m *MockProductRepository) BulkArchiveProducts(productIDs []uint, userID uint) []database.BulkOperationError {
	return nil
}
func (m *MockProductRepository) RestoreProduct(productID, userID uint) error { return m.Err }
func (m *MockProductRepository) BulkRestoreProducts(productIDs []uint, userID uint) []database.BulkOperationError {
	return nil
}
func (m *MockProductRepository) SetProductExpireAt(productID uint, userID uint, expireAt dbModel.Timestamp) error {
	return m.Err
}
func (m *MockProductRepository) SetProductNotifiedAt(productID uint) error { return m.Err }
func (m *MockProductRepository) GetProductsExpired(userID uint) ([]*dbModel.Product, error) {
	ptrs := make([]*dbModel.Product, len(m.Products))
	for i := range m.Products {
		ptrs[i] = &m.Products[i]
	}
	return ptrs, m.Err
}
func (m *MockProductRepository) GetExpiredProductsCount(userID uint) (int, error) {
	return m.IntResult, m.Err
}
func (m *MockProductRepository) GetArchivedProductsGroupedByBarcode(userID uint) (map[string]int, error) {
	return m.MapResult, m.Err
}
func (m *MockProductRepository) GetTopArchivedProducts(userID uint, limit int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetActiveProductsCount(userID uint) (int, error) {
	return m.IntResult, m.Err
}
func (m *MockProductRepository) GetProductCategoryBreakdown(userID uint) (map[string]int, error) {
	return m.MapResult, m.Err
}
func (m *MockProductRepository) GetExpiryTrend(userID uint) ([]apiModel.StatsMonthlyCount, error) {
	return m.StatsMonthly, m.Err
}
func (m *MockProductRepository) GetExpiringSoonProducts(userID uint, days int) ([]apiModel.StatsExpiringProduct, error) {
	return m.ExpiringProds, m.Err
}
func (m *MockProductRepository) GetLastNotifiedProduct(householdID uint) (dbModel.Product, error) {
	return m.Product, m.Err
}
func (m *MockProductRepository) GetExpiringInDays(userID uint, days int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetLastInsertedProduct(householdID uint) (dbModel.Product, error) {
	return m.Product, m.Err
}
func (m *MockProductRepository) UserHasProductAccess(userID uint, productID int) bool {
	return m.BoolResult
}
func (m *MockProductRepository) GetOpenFoodFactsCacheByBarcode(barcode string) (dbModel.OpenFoodFactsCache, error) {
	return m.OFFCache, m.Err
}
func (m *MockProductRepository) CreateOpenFoodFactsCache(entry *dbModel.OpenFoodFactsCache) error {
	return m.Err
}
func (m *MockProductRepository) UpdateOpenFoodFactsCacheImageURL(barcode, imageURL string) error {
	return m.Err
}
func (m *MockProductRepository) GetUserByID(userID uint) (authentication.User, error) {
	return m.User, m.Err
}
func (m *MockProductRepository) GetUserHouseholdByID(userID uint) (uint, error) {
	return m.User.HouseholdID, m.Err
}
func (m *MockProductRepository) GetHouseholdByID(householdID uint) (dbModel.Household, error) {
	return m.Household, m.Err
}
func (m *MockProductRepository) GetUserActiveProductsFiltered(userID uint, from, to *time.Time) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetUserArchivedProductsFiltered(userID uint, from, to *time.Time) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetUsersByHouseholdID(householdID uint) ([]authentication.User, error) {
	return m.Users, m.Err
}
func (m *MockProductRepository) GetExpiringSoonCount(userID uint, days int) (int, error) {
	return m.IntResult, m.Err
}
func (m *MockProductRepository) GetWasteThisMonth(userID uint) (int, error) {
	return m.IntResult, m.Err
}
func (m *MockProductRepository) GetExpiringProductsByHousehold(householdID uint, daysAhead int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) GetProductsByHousehold(householdID uint) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockProductRepository) ConsumeProduct(productID, userID uint) error { return m.Err }
func (m *MockProductRepository) WasteProduct(productID, userID uint) error   { return m.Err }
func (m *MockProductRepository) BulkConsumeProducts(productIDs []uint, userID uint) []database.BulkOperationError {
	return nil
}
func (m *MockProductRepository) BulkWasteProducts(productIDs []uint, userID uint) []database.BulkOperationError {
	return nil
}

var _ database.ProductRepositoryInterface = (*MockProductRepository)(nil)

// MockUserRepository is a configurable in-memory stub for UserRepositoryInterface.
type MockUserRepository struct {
	User                    authentication.User
	Users                   []authentication.User
	Household               dbModel.Household
	OnboardingState         dbModel.OnboardingState
	EmailVerification       dbModel.EmailVerification
	HouseholdID             uint
	UsernameExistsResult    bool
	MailAddressExistsResult bool
	LockDuration            time.Duration
	IsLocked                bool
	Err                     error
}

func (m *MockUserRepository) GetUserByUsername(username string) (authentication.User, error) {
	return m.User, m.Err
}
func (m *MockUserRepository) GetUserByID(userID uint) (authentication.User, error) {
	return m.User, m.Err
}
func (m *MockUserRepository) GetUserHouseholdByID(userID uint) (uint, error) {
	return m.HouseholdID, m.Err
}
func (m *MockUserRepository) UserExistsByUsername(user *authentication.User) bool {
	return m.UsernameExistsResult
}
func (m *MockUserRepository) UserExistsByMailAddress(user *authentication.User) bool {
	return m.MailAddressExistsResult
}
func (m *MockUserRepository) CreateUser(user *authentication.User) error { return m.Err }
func (m *MockUserRepository) UpdateUser(userID uint, user *authentication.User) error {
	return m.Err
}
func (m *MockUserRepository) UpdateAdminUserFields(userID uint, username, mailAddress string) error {
	return m.Err
}
func (m *MockUserRepository) UpdateDisplayName(userID uint, displayName string) error { return m.Err }
func (m *MockUserRepository) UpdateUserPassword(userID uint, login *authentication.Login) error {
	return m.Err
}
func (m *MockUserRepository) IsAccountLocked(userID uint, maxLoginAttempts int, lockoutDurationMins int) (bool, time.Duration) {
	return m.IsLocked, m.LockDuration
}
func (m *MockUserRepository) RecordFailedLoginAttempt(userID uint, maxLoginAttempts int, lockoutDurationMins int) error {
	return m.Err
}
func (m *MockUserRepository) ResetFailedLoginAttempts(userID uint) error { return m.Err }
func (m *MockUserRepository) CreateEmailVerification(userID uint, token string, expiresAt time.Time) error {
	return m.Err
}
func (m *MockUserRepository) GetEmailVerificationByToken(token string) (dbModel.EmailVerification, error) {
	return m.EmailVerification, m.Err
}
func (m *MockUserRepository) UpdateUserEmailVerified(userID uint, verifiedAt time.Time) error {
	return m.Err
}
func (m *MockUserRepository) UpdateEmailVerificationStatus(token, status string) error {
	return m.Err
}
func (m *MockUserRepository) GetOnboardingState(userID uint) (dbModel.OnboardingState, error) {
	return m.OnboardingState, m.Err
}
func (m *MockUserRepository) MarkNotificationsSetup(userID uint) error          { return m.Err }
func (m *MockUserRepository) UpdateUsername(userID uint, username string) error { return m.Err }
func (m *MockUserRepository) MarkProfileStepDone(userID uint) error             { return m.Err }
func (m *MockUserRepository) MarkHouseholdStepDone(userID uint) error           { return m.Err }
func (m *MockUserRepository) MarkOnboardingComplete(userID uint) error          { return m.Err }
func (m *MockUserRepository) EnsureOnboardingState(userID uint) error           { return m.Err }
func (m *MockUserRepository) GetHouseholdByID(householdID uint) (dbModel.Household, error) {
	return m.Household, m.Err
}
func (m *MockUserRepository) GetUsersByHouseholdID(householdID uint) ([]authentication.User, error) {
	return m.Users, m.Err
}
func (m *MockUserRepository) DeleteUser(userID uint) error { return m.Err }

var _ database.UserRepositoryInterface = (*MockUserRepository)(nil)

// MockHouseholdRepository is a configurable in-memory stub for HouseholdRepositoryInterface.
type MockHouseholdRepository struct {
	Household    dbModel.Household
	Households   []dbModel.HouseholdWithMemberCount
	Users        []authentication.User
	Applications []dbModel.HouseholdApplication
	MemberCount  int64
	Err          error
}

func (m *MockHouseholdRepository) GetHouseholdByID(householdID uint) (dbModel.Household, error) {
	return m.Household, m.Err
}
func (m *MockHouseholdRepository) GetHouseholdMemberCount(householdID uint) (int64, error) {
	return m.MemberCount, m.Err
}
func (m *MockHouseholdRepository) GetHouseholdMembers(householdID uint) ([]authentication.User, error) {
	return m.Users, m.Err
}
func (m *MockHouseholdRepository) LeaveHousehold(userID uint) error { return m.Err }
func (m *MockHouseholdRepository) CreateAndSwitchHousehold(userID uint, name string) error {
	return m.Err
}
func (m *MockHouseholdRepository) ApplyForHousehold(applicantID, householdID uint) error {
	return m.Err
}
func (m *MockHouseholdRepository) GetPendingApplicationsForAdmin(adminUserID uint) ([]dbModel.HouseholdApplication, error) {
	return m.Applications, m.Err
}
func (m *MockHouseholdRepository) ApproveApplication(applicationID, adminUserID uint) error {
	return m.Err
}
func (m *MockHouseholdRepository) RejectApplication(applicationID, adminUserID uint) error {
	return m.Err
}
func (m *MockHouseholdRepository) GetPendingApplicationsForApplicant(applicantUserID uint) ([]dbModel.HouseholdApplication, error) {
	return m.Applications, m.Err
}
func (m *MockHouseholdRepository) CancelApplication(applicationID, applicantUserID uint) error {
	return m.Err
}
func (m *MockHouseholdRepository) UpdateHouseholdName(householdID, adminUserID uint, name string) error {
	return m.Err
}
func (m *MockHouseholdRepository) RemoveMemberFromHousehold(memberUserID, adminUserID uint) error {
	return m.Err
}
func (m *MockHouseholdRepository) GetPublicHouseholds(excludeHouseholdID uint) ([]dbModel.HouseholdWithMemberCount, error) {
	return m.Households, m.Err
}

var _ database.HouseholdRepositoryInterface = (*MockHouseholdRepository)(nil)

// MockInvitationRepository is a configurable in-memory stub for InvitationRepositoryInterface.
type MockInvitationRepository struct {
	Invitation  dbModel.HouseholdInvitation
	Invitations []dbModel.HouseholdInvitation
	Err         error
}

func (m *MockInvitationRepository) CreateInvitation(householdID, inviterID uint, email string) (dbModel.HouseholdInvitation, error) {
	return m.Invitation, m.Err
}
func (m *MockInvitationRepository) GetInvitationsForHousehold(householdID, inviterID uint) ([]dbModel.HouseholdInvitation, error) {
	return m.Invitations, m.Err
}
func (m *MockInvitationRepository) GetPendingInvitationsForHousehold(householdID uint) ([]dbModel.HouseholdInvitation, error) {
	return m.Invitations, m.Err
}
func (m *MockInvitationRepository) GetInvitationByToken(token string) (dbModel.HouseholdInvitation, error) {
	return m.Invitation, m.Err
}
func (m *MockInvitationRepository) AcceptInvitation(token, email string, userID uint) error {
	return m.Err
}
func (m *MockInvitationRepository) CancelInvitation(invitationID, userID uint) error { return m.Err }
func (m *MockInvitationRepository) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]dbModel.HouseholdInvitation, error) {
	return m.Invitations, m.Err
}
func (m *MockInvitationRepository) MarkInvitationSent(invitationID uint) error       { return m.Err }
func (m *MockInvitationRepository) MarkInvitationSendFailed(invitationID uint) error { return m.Err }
func (m *MockInvitationRepository) MarkInvitationExpired(invitationID uint) error    { return m.Err }

var _ database.InvitationRepositoryInterface = (*MockInvitationRepository)(nil)

// MockStorageLocationRepository is a configurable in-memory stub for StorageLocationRepositoryInterface.
type MockStorageLocationRepository struct {
	Location  dbModel.StorageLocation
	Locations []dbModel.StorageLocation
	Err       error
}

func (m *MockStorageLocationRepository) GetByHousehold(userID uint) ([]dbModel.StorageLocation, error) {
	return m.Locations, m.Err
}
func (m *MockStorageLocationRepository) GetByID(locationID, userID uint) (dbModel.StorageLocation, error) {
	return m.Location, m.Err
}
func (m *MockStorageLocationRepository) Create(userID uint, name, icon string, sortOrder int) (dbModel.StorageLocation, error) {
	return m.Location, m.Err
}
func (m *MockStorageLocationRepository) Update(locationID, userID uint, name, icon string, sortOrder int) (dbModel.StorageLocation, error) {
	return m.Location, m.Err
}
func (m *MockStorageLocationRepository) Delete(locationID, userID uint) error { return m.Err }

var _ database.StorageLocationRepositoryInterface = (*MockStorageLocationRepository)(nil)

// MockWebhookRepository is a configurable in-memory stub for WebhookRepositoryInterface.
type MockWebhookRepository struct {
	Webhook  dbModel.Webhook
	Webhooks []dbModel.Webhook
	Logs     []dbModel.WebhookDeliveryLog
	Err      error
}

func (m *MockWebhookRepository) CreateWebhook(webhook *dbModel.Webhook) error { return m.Err }
func (m *MockWebhookRepository) GetWebhooksByUserID(userID uint) ([]dbModel.Webhook, error) {
	return m.Webhooks, m.Err
}
func (m *MockWebhookRepository) GetWebhookByID(webhookID uint) (dbModel.Webhook, error) {
	return m.Webhook, m.Err
}
func (m *MockWebhookRepository) GetActiveWebhooksByEvent(event string) ([]dbModel.Webhook, error) {
	return m.Webhooks, m.Err
}
func (m *MockWebhookRepository) UpdateWebhook(webhook *dbModel.Webhook) error { return m.Err }
func (m *MockWebhookRepository) DeleteWebhook(webhookID uint) error           { return m.Err }
func (m *MockWebhookRepository) CreateDeliveryLog(log *dbModel.WebhookDeliveryLog) error {
	return m.Err
}
func (m *MockWebhookRepository) GetDeliveryLogs(webhookID uint, limit int) ([]dbModel.WebhookDeliveryLog, error) {
	return m.Logs, m.Err
}
func (m *MockWebhookRepository) TrimDeliveryLogs(webhookID uint, keep int) error { return m.Err }
func (m *MockWebhookRepository) CheckOwnership(webhookID, userID uint) error     { return m.Err }

var _ database.WebhookRepositoryInterface = (*MockWebhookRepository)(nil)

// MockPATRepository is a configurable in-memory stub for PATRepositoryInterface.
type MockPATRepository struct {
	PAT  *authentication.PersonalAccessToken
	PATs []authentication.PersonalAccessToken
	Err  error
}

func (m *MockPATRepository) CreatePAT(userID uint, name, tokenHash string, expiresAt *time.Time, scopes string) (*authentication.PersonalAccessToken, error) {
	return m.PAT, m.Err
}
func (m *MockPATRepository) GetPATByTokenHash(tokenHash string) (*authentication.PersonalAccessToken, error) {
	return m.PAT, m.Err
}
func (m *MockPATRepository) GetPATsByUserID(userID uint) ([]authentication.PersonalAccessToken, error) {
	return m.PATs, m.Err
}
func (m *MockPATRepository) GetPATByID(patID uint) (*authentication.PersonalAccessToken, error) {
	return m.PAT, m.Err
}
func (m *MockPATRepository) DeletePAT(patID, userID uint) error { return m.Err }
func (m *MockPATRepository) UpdateLastUsed(patID uint) error    { return m.Err }

var _ database.PATRepositoryInterface = (*MockPATRepository)(nil)

// MockSavingsRepository is a configurable in-memory stub for SavingsRepositoryInterface.
type MockSavingsRepository struct {
	CategoryPrice *dbModel.ProductCategoryPrice
	SavingsStats  apiModel.SavingsStatsResponse
	Err           error
}

func (m *MockSavingsRepository) MatchCategory(categories string) (*dbModel.ProductCategoryPrice, error) {
	return m.CategoryPrice, m.Err
}
func (m *MockSavingsRepository) RecordSavingsEvent(householdID uint, product *dbModel.Product, eventType string) error {
	return m.Err
}
func (m *MockSavingsRepository) GetSavingsStats(householdID uint) (apiModel.SavingsStatsResponse, error) {
	return m.SavingsStats, m.Err
}

var _ database.SavingsRepositoryInterface = (*MockSavingsRepository)(nil)

// MockRecipeRepository is a configurable in-memory stub for RecipeRepositoryInterface.
type MockRecipeRepository struct {
	Cache dbModel.RecipeCache
	Err   error
}

func (m *MockRecipeRepository) GetCacheByQueryHash(hash string) (dbModel.RecipeCache, error) {
	return m.Cache, m.Err
}
func (m *MockRecipeRepository) CreateCache(cache *dbModel.RecipeCache) error { return m.Err }
func (m *MockRecipeRepository) UpdateCacheHit(hash string) error             { return m.Err }
func (m *MockRecipeRepository) CleanupExpiredCaches() error                  { return m.Err }

var _ database.RecipeRepositoryInterface = (*MockRecipeRepository)(nil)

// MockNotificationRepository is a configurable in-memory stub for NotificationRepositoryInterface.
type MockNotificationRepository struct {
	Products            []dbModel.Product
	User                authentication.User
	Users               []authentication.User
	Household           dbModel.Household
	HouseholdTargets    []models.HouseholdReportTarget
	NotifRecipients     []models.NotificationRecipientInfo
	MailAddresses       []string
	Invitation          dbModel.HouseholdInvitation
	Invitations         []dbModel.HouseholdInvitation
	OnboardingState     dbModel.OnboardingState
	HouseholdsWithCount []dbModel.HouseholdWithMemberCount
	WasteStats          models.WasteStats
	MaxThresholdDays    int
	Err                 error
}

func (m *MockNotificationRepository) GetProductsExpiredAndNotificationPending(sleepInterval time.Duration, maxLookAheadDays int) ([]dbModel.Product, error) {
	return m.Products, m.Err
}
func (m *MockNotificationRepository) GetMaxNotificationThresholdDays() int { return m.MaxThresholdDays }
func (m *MockNotificationRepository) GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error) {
	return m.MailAddresses, m.Err
}
func (m *MockNotificationRepository) GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error) {
	return m.NotifRecipients, m.Err
}
func (m *MockNotificationRepository) SetProductNotifiedAt(productID uint) error { return m.Err }
func (m *MockNotificationRepository) CreateInvitation(householdID, inviterID uint, email string) (dbModel.HouseholdInvitation, error) {
	return m.Invitation, m.Err
}
func (m *MockNotificationRepository) GetInvitationsForHousehold(householdID, inviterID uint) ([]dbModel.HouseholdInvitation, error) {
	return m.Invitations, m.Err
}
func (m *MockNotificationRepository) GetInvitationByToken(token string) (dbModel.HouseholdInvitation, error) {
	return m.Invitation, m.Err
}
func (m *MockNotificationRepository) AcceptInvitation(token, email string, userID uint) error {
	return m.Err
}
func (m *MockNotificationRepository) CancelInvitation(invitationID, userID uint) error {
	return m.Err
}
func (m *MockNotificationRepository) GetUserByID(userID uint) (authentication.User, error) {
	return m.User, m.Err
}
func (m *MockNotificationRepository) GetHouseholdByID(householdID uint) (dbModel.Household, error) {
	return m.Household, m.Err
}
func (m *MockNotificationRepository) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]dbModel.HouseholdInvitation, error) {
	return m.Invitations, m.Err
}
func (m *MockNotificationRepository) MarkInvitationSent(invitationID uint) error { return m.Err }
func (m *MockNotificationRepository) MarkInvitationSendFailed(invitationID uint) error {
	return m.Err
}
func (m *MockNotificationRepository) GetHouseholdsWithMonthlyWasteReportEnabled() ([]models.HouseholdReportTarget, error) {
	return m.HouseholdTargets, m.Err
}
func (m *MockNotificationRepository) GetWasteStatsForHousehold(householdID uint, month time.Time) (models.WasteStats, error) {
	return m.WasteStats, m.Err
}
func (m *MockNotificationRepository) GetOnboardingState(userID uint) (dbModel.OnboardingState, error) {
	return m.OnboardingState, m.Err
}
func (m *MockNotificationRepository) MarkNotificationsSetup(userID uint) error { return m.Err }
func (m *MockNotificationRepository) MarkHouseholdStepDone(userID uint) error  { return m.Err }
func (m *MockNotificationRepository) MarkOnboardingComplete(userID uint) error { return m.Err }
func (m *MockNotificationRepository) GetPublicHouseholds(excludeHouseholdID uint) ([]dbModel.HouseholdWithMemberCount, error) {
	return m.HouseholdsWithCount, m.Err
}
func (m *MockNotificationRepository) FindUserByTelegramLinkToken(token string) (authentication.User, error) {
	return m.User, m.Err
}
func (m *MockNotificationRepository) SetTelegramChatID(userID uint, chatID string) error {
	return m.Err
}
func (m *MockNotificationRepository) SetTelegramLinkToken(userID uint, token string) error {
	return m.Err
}
func (m *MockNotificationRepository) SetTelegramBotUsername(userID uint, username string) error {
	return m.Err
}
func (m *MockNotificationRepository) GetAllUsersWithTelegramBotToken() ([]authentication.User, error) {
	return m.Users, m.Err
}

var _ database.NotificationRepositoryInterface = (*MockNotificationRepository)(nil)

// MockStreakRepository is a configurable in-memory stub for StreakRepositoryInterface.
type MockStreakRepository struct {
	Streak  *dbModel.WasteStreak
	Streaks []dbModel.WasteStreak
	Err     error
}

func (m *MockStreakRepository) GetOrCreateStreakForHousehold(householdID uint) (*dbModel.WasteStreak, error) {
	return m.Streak, m.Err
}
func (m *MockStreakRepository) RecordWasteEvent(householdID uint) error        { return m.Err }
func (m *MockStreakRepository) UpdateStreak(streak *dbModel.WasteStreak) error { return m.Err }
func (m *MockStreakRepository) GetAllStreaks() ([]dbModel.WasteStreak, error) {
	return m.Streaks, m.Err
}

var _ database.StreakRepositoryInterface = (*MockStreakRepository)(nil)

// MockExpiryScanRepository is a configurable in-memory stub for ExpiryScanRepositoryInterface.
type MockExpiryScanRepository struct {
	Scans []dbModel.ExpiryScan
	Err   error
}

func (m *MockExpiryScanRepository) Create(scan *dbModel.ExpiryScan) error { return m.Err }
func (m *MockExpiryScanRepository) GetByUser(userID uint, limit int) ([]dbModel.ExpiryScan, error) {
	return m.Scans, m.Err
}

var _ database.ExpiryScanRepositoryInterface = (*MockExpiryScanRepository)(nil)

// MockCalendarTokenRepository is a configurable in-memory stub for CalendarTokenRepositoryInterface.
type MockCalendarTokenRepository struct {
	CalendarToken authentication.CalendarToken
	Err           error
}

func (m *MockCalendarTokenRepository) GetByToken(token string) (authentication.CalendarToken, error) {
	return m.CalendarToken, m.Err
}
func (m *MockCalendarTokenRepository) DeleteByUserID(userID uint) error { return m.Err }
func (m *MockCalendarTokenRepository) GetByUserID(userID uint) (authentication.CalendarToken, error) {
	return m.CalendarToken, m.Err
}
func (m *MockCalendarTokenRepository) Create(ct *authentication.CalendarToken) error { return m.Err }

var _ database.CalendarTokenRepositoryInterface = (*MockCalendarTokenRepository)(nil)

// MockRepositoryContainer holds mock implementations of all repository interfaces.
type MockRepositoryContainer struct {
	Products         *MockProductRepository
	Users            *MockUserRepository
	Households       *MockHouseholdRepository
	Invitations      *MockInvitationRepository
	StorageLocations *MockStorageLocationRepository
	Webhooks         *MockWebhookRepository
	PATs             *MockPATRepository
	Savings          *MockSavingsRepository
	Recipes          *MockRecipeRepository
	Notifications    *MockNotificationRepository
	Streaks          *MockStreakRepository
	ExpiryScan       *MockExpiryScanRepository
	CalendarTokens   *MockCalendarTokenRepository
}

// NewMockRepositoryContainer creates a MockRepositoryContainer with all mocks initialised.
func NewMockRepositoryContainer() *MockRepositoryContainer {
	return &MockRepositoryContainer{
		Products:         &MockProductRepository{},
		Users:            &MockUserRepository{},
		Households:       &MockHouseholdRepository{},
		Invitations:      &MockInvitationRepository{},
		StorageLocations: &MockStorageLocationRepository{},
		Webhooks:         &MockWebhookRepository{},
		PATs:             &MockPATRepository{},
		Savings:          &MockSavingsRepository{},
		Recipes:          &MockRecipeRepository{},
		Notifications:    &MockNotificationRepository{},
		Streaks:          &MockStreakRepository{},
		ExpiryScan:       &MockExpiryScanRepository{},
		CalendarTokens:   &MockCalendarTokenRepository{},
	}
}

// ToRepositoryContainer converts the mock container to a database.RepositoryContainer
// suitable for injection into the Gin context via ctx.Set(util.ContextKeyRepos, ...).
func (m *MockRepositoryContainer) ToRepositoryContainer() *database.RepositoryContainer {
	return &database.RepositoryContainer{
		Products:         m.Products,
		Users:            m.Users,
		Households:       m.Households,
		Invitations:      m.Invitations,
		StorageLocations: m.StorageLocations,
		Webhooks:         m.Webhooks,
		PATs:             m.PATs,
		Recipes:          m.Recipes,
		Savings:          m.Savings,
		Notifications:    m.Notifications,
		Streaks:          m.Streaks,
		ExpiryScan:       m.ExpiryScan,
		CalendarTokens:   m.CalendarTokens,
	}
}
