package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
)

// ShoppingListSyncPayload represents the normalized state of a local shopping list item for external synchronization.
type ShoppingListSyncPayload struct {
	ID       uint
	Name     string
	Category string
	Quantity int
	Unit     string
	Checked  bool
	Notes    string
}

// ShoppingListPushResult summarizes item references created, updated, or deleted during a provider push.
type ShoppingListPushResult struct {
	Created map[uint]string
	Updated []uint
	Deleted []string
}

// ShoppingListSyncProvider defines the adapter contract for external shopping list providers.
type ShoppingListSyncProvider interface {
	Name() string
	TestConnection(ctx context.Context, integration *dbModel.HouseholdIntegration) error
	Push(ctx context.Context, integration *dbModel.HouseholdIntegration, items []ShoppingListSyncPayload, refs []dbModel.ShoppingListItemSyncRef) (ShoppingListPushResult, error)
	PullChecked(ctx context.Context, integration *dbModel.HouseholdIntegration, refs []dbModel.ShoppingListItemSyncRef) (map[string]bool, error)
}

// ShoppingListSyncProviderFactory constructs a ShoppingListSyncProvider for a given provider identifier.
type ShoppingListSyncProviderFactory func(provider string, integration *dbModel.HouseholdIntegration, logger *zerolog.Logger, client *http.Client) (ShoppingListSyncProvider, error)

// NewShoppingListSyncProvider builds the shopping list sync provider for the given provider identifier.
func NewShoppingListSyncProvider(provider string, _ *dbModel.HouseholdIntegration, logger *zerolog.Logger, client *http.Client) (ShoppingListSyncProvider, error) {
	if client == nil {
		client = &http.Client{
			Timeout: time.Duration(util.DefaultShoppingListSyncTimeoutSeconds) * time.Second,
		}
	}
	switch provider {
	case util.ShoppingListProviderNoop:
		return &NoopShoppingListSyncProvider{}, nil
	case util.ShoppingListProviderTodoist:
		return &TodoistSyncProvider{
			Logger:     logger,
			HTTPClient: client,
			BaseURL:    util.DefaultTodoistAPIBaseURL,
		}, nil
	case util.ShoppingListProviderBring:
		return &BringSyncProvider{
			Logger:     logger,
			HTTPClient: client,
			BaseURL:    util.DefaultBringAPIBaseURL,
		}, nil
	default:
		return nil, errors.ErrIntegrationInvalidProvider
	}
}

// NoopShoppingListSyncProvider is an in-memory provider used for testing and offline validation without a live external account.
type NoopShoppingListSyncProvider struct {
	mu           sync.Mutex
	TestErr      error
	PushErr      error
	PullErr      error
	CheckedState map[string]bool
	LastPushed   []ShoppingListSyncPayload
	PushCount    int
	PullCount    int
	TestCount    int
}

// Name returns the provider identifier.
func (p *NoopShoppingListSyncProvider) Name() string {
	return util.ShoppingListProviderNoop
}

// TestConnection increments TestCount and returns TestErr.
func (p *NoopShoppingListSyncProvider) TestConnection(_ context.Context, integration *dbModel.HouseholdIntegration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TestCount++
	if integration == nil {
		return errors.ErrIntegrationMissingCredentials
	}
	return p.TestErr
}

// Push records the pushed items and computes deterministic external IDs.
func (p *NoopShoppingListSyncProvider) Push(_ context.Context, _ *dbModel.HouseholdIntegration, items []ShoppingListSyncPayload, refs []dbModel.ShoppingListItemSyncRef) (ShoppingListPushResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.PushCount++
	if p.PushErr != nil {
		return ShoppingListPushResult{}, p.PushErr
	}

	p.LastPushed = make([]ShoppingListSyncPayload, len(items))
	copy(p.LastPushed, items)

	refByItemID := make(map[uint]string, len(refs))
	for i := range refs {
		if refs[i].ShoppingListItemID > 0 {
			refByItemID[refs[i].ShoppingListItemID] = refs[i].ExternalID
		}
	}

	seenItemID := make(map[uint]bool, len(items))
	result := ShoppingListPushResult{
		Created: make(map[uint]string),
	}
	for i := range items {
		item := items[i]
		seenItemID[item.ID] = true
		if _, exists := refByItemID[item.ID]; exists {
			result.Updated = append(result.Updated, item.ID)
			continue
		}
		result.Created[item.ID] = "noop-" + strconv.FormatUint(uint64(item.ID), 10)
	}

	for i := range refs {
		if refs[i].ShoppingListItemID == 0 || !seenItemID[refs[i].ShoppingListItemID] {
			result.Deleted = append(result.Deleted, refs[i].ExternalID)
		}
	}

	return result, nil
}

// PullChecked returns the configured CheckedState for tracked references.
func (p *NoopShoppingListSyncProvider) PullChecked(_ context.Context, _ *dbModel.HouseholdIntegration, refs []dbModel.ShoppingListItemSyncRef) (map[string]bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.PullCount++
	if p.PullErr != nil {
		return nil, p.PullErr
	}

	out := make(map[string]bool, len(refs))
	for i := range refs {
		extID := refs[i].ExternalID
		if p.CheckedState != nil {
			if checked, exists := p.CheckedState[extID]; exists {
				out[extID] = checked
			}
		}
	}
	return out, nil
}

// TodoistSyncProvider synchronizes a household shopping list with Todoist REST API v2.
type TodoistSyncProvider struct {
	Logger     *zerolog.Logger
	HTTPClient *http.Client
	BaseURL    string
}

type todoistTaskCreateRequest struct {
	Content     string `json:"content"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
}

type todoistTaskUpdateRequest struct {
	Content     string `json:"content"`
	Description string `json:"description"`
}

type todoistTaskResponse struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	IsCompleted bool   `json:"is_completed"`
}

// Name returns the Todoist provider identifier.
func (p *TodoistSyncProvider) Name() string {
	return util.ShoppingListProviderTodoist
}

func (p *TodoistSyncProvider) baseURL() string {
	if strings.TrimSpace(p.BaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	}
	return util.DefaultTodoistAPIBaseURL
}

func (p *TodoistSyncProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: time.Duration(util.DefaultShoppingListSyncTimeoutSeconds) * time.Second}
}

func formatTodoistTaskContent(item *ShoppingListSyncPayload) string {
	var b strings.Builder
	b.WriteString(item.Name)
	unit := strings.TrimSpace(item.Unit)
	if item.Quantity > 1 || unit != "" {
		b.WriteString(" (")
		b.WriteString(strconv.Itoa(max(1, item.Quantity)))
		if unit != "" {
			b.WriteByte(' ')
			b.WriteString(unit)
		}
		b.WriteByte(')')
	}
	return b.String()
}

func (p *TodoistSyncProvider) doRequest(ctx context.Context, token, method, endpoint string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set(util.RequestHeaderContentType, "application/json")
	}
	return p.client().Do(req)
}

// TestConnection verifies the Todoist API token and optional project ID.
func (p *TodoistSyncProvider) TestConnection(ctx context.Context, integration *dbModel.HouseholdIntegration) error {
	if integration == nil || strings.TrimSpace(integration.AccessToken) == "" {
		return errors.ErrIntegrationMissingCredentials
	}
	endpoint := p.baseURL() + "/projects"
	if listID := strings.TrimSpace(integration.ListID); listID != "" {
		endpoint = p.baseURL() + "/projects/" + url.PathEscape(listID)
	}
	resp, err := p.doRequest(ctx, strings.TrimSpace(integration.AccessToken), http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}
	return nil
}

// Push synchronizes local shopping list items to Todoist tasks.
func (p *TodoistSyncProvider) Push(ctx context.Context, integration *dbModel.HouseholdIntegration, items []ShoppingListSyncPayload, refs []dbModel.ShoppingListItemSyncRef) (ShoppingListPushResult, error) {
	if integration == nil || strings.TrimSpace(integration.AccessToken) == "" {
		return ShoppingListPushResult{}, errors.ErrIntegrationMissingCredentials
	}
	token := strings.TrimSpace(integration.AccessToken)
	projectID := strings.TrimSpace(integration.ListID)

	refByItemID := make(map[uint]string, len(refs))
	for i := range refs {
		if refs[i].ShoppingListItemID > 0 {
			refByItemID[refs[i].ShoppingListItemID] = refs[i].ExternalID
		}
	}

	seenItemID := make(map[uint]bool, len(items))
	result := ShoppingListPushResult{
		Created: make(map[uint]string),
	}

	for i := range items {
		item := &items[i]
		seenItemID[item.ID] = true
		extID, exists := refByItemID[item.ID]
		if exists {
			recreatedID, wasRecreated, err := p.updateTodoistTask(ctx, token, projectID, extID, item)
			if err != nil {
				return result, err
			}
			if wasRecreated {
				result.Deleted = append(result.Deleted, extID)
				result.Created[item.ID] = recreatedID
			} else {
				result.Updated = append(result.Updated, item.ID)
			}
			continue
		}

		createdID, err := p.createTodoistTask(ctx, token, projectID, item)
		if err != nil {
			return result, err
		}
		result.Created[item.ID] = createdID
	}

	for i := range refs {
		if refs[i].ShoppingListItemID == 0 || !seenItemID[refs[i].ShoppingListItemID] {
			if err := p.deleteTodoistTask(ctx, token, refs[i].ExternalID); err != nil {
				return result, err
			}
			result.Deleted = append(result.Deleted, refs[i].ExternalID)
		}
	}

	return result, nil
}

func (p *TodoistSyncProvider) createTodoistTask(ctx context.Context, token, projectID string, item *ShoppingListSyncPayload) (string, error) {
	reqBody := todoistTaskCreateRequest{
		Content:     formatTodoistTaskContent(item),
		Description: item.Notes,
		ProjectID:   projectID,
	}
	resp, err := p.doRequest(ctx, token, http.MethodPost, p.baseURL()+"/tasks", reqBody)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
		return "", fmt.Errorf("%w: create task status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}
	var taskResp todoistTaskResponse
	if err := decodeJSONResponse(resp, &taskResp); err != nil {
		return "", fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}
	if taskResp.ID == "" {
		return "", errors.ErrIntegrationSyncFailed
	}
	if item.Checked {
		if err := p.setTodoistTaskCompletion(ctx, token, taskResp.ID, true); err != nil {
			return "", err
		}
	}
	return taskResp.ID, nil
}

func (p *TodoistSyncProvider) updateTodoistTask(ctx context.Context, token, projectID, extID string, item *ShoppingListSyncPayload) (string, bool, error) {
	reqBody := todoistTaskUpdateRequest{
		Content:     formatTodoistTaskContent(item),
		Description: item.Notes,
	}
	endpoint := p.baseURL() + "/tasks/" + url.PathEscape(extID)
	resp, err := p.doRequest(ctx, token, http.MethodPost, endpoint, reqBody)
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))

	if resp.StatusCode == http.StatusNotFound {
		createdID, createErr := p.createTodoistTask(ctx, token, projectID, item)
		if createErr != nil {
			return "", false, createErr
		}
		return createdID, true, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", false, fmt.Errorf("%w: update task status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}

	if err := p.setTodoistTaskCompletion(ctx, token, extID, item.Checked); err != nil {
		return "", false, err
	}
	return extID, false, nil
}

func (p *TodoistSyncProvider) setTodoistTaskCompletion(ctx context.Context, token, extID string, completed bool) error {
	action := "reopen"
	if completed {
		action = "close"
	}
	endpoint := p.baseURL() + "/tasks/" + url.PathEscape(extID) + "/" + action
	resp, err := p.doRequest(ctx, token, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %s task status %d", errors.ErrIntegrationSyncFailed, action, resp.StatusCode)
	}
	return nil
}

func (p *TodoistSyncProvider) deleteTodoistTask(ctx context.Context, token, extID string) error {
	endpoint := p.baseURL() + "/tasks/" + url.PathEscape(extID)
	resp, err := p.doRequest(ctx, token, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: delete task status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}
	return nil
}

// PullChecked retrieves task completion states from Todoist for tracked item references.
func (p *TodoistSyncProvider) PullChecked(ctx context.Context, integration *dbModel.HouseholdIntegration, refs []dbModel.ShoppingListItemSyncRef) (map[string]bool, error) {
	if integration == nil || strings.TrimSpace(integration.AccessToken) == "" {
		return nil, errors.ErrIntegrationMissingCredentials
	}
	if len(refs) == 0 {
		return map[string]bool{}, nil
	}
	token := strings.TrimSpace(integration.AccessToken)
	activeTasks := make(map[string]bool)
	fullyFetched := true

	limit := 200
	offset := 0
	for {
		endpoint := p.baseURL() + "/tasks?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset)
		if projectID := strings.TrimSpace(integration.ListID); projectID != "" {
			endpoint = p.baseURL() + "/tasks?project_id=" + url.QueryEscape(projectID) + "&limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset)
		}

		resp, err := p.doRequest(ctx, token, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%w: list tasks status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
		}

		var tasks []todoistTaskResponse
		if err := decodeJSONResponse(resp, &tasks); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
		}
		_ = resp.Body.Close()

		for i := range tasks {
			activeTasks[tasks[i].ID] = tasks[i].IsCompleted
		}
		if len(tasks) < limit {
			break
		}
		offset += len(tasks)
		if offset > 10000 {
			fullyFetched = false
			break
		}
	}

	checked := make(map[string]bool, len(refs))
	for i := range refs {
		extID := refs[i].ExternalID
		if isCompleted, exists := activeTasks[extID]; exists {
			checked[extID] = isCompleted
		} else if fullyFetched {
			checked[extID] = true
		}
	}
	return checked, nil
}

// BringSyncProvider synchronizes a household shopping list with Bring!.
type BringSyncProvider struct {
	Logger     *zerolog.Logger
	HTTPClient *http.Client
	BaseURL    string
}

type bringItemPayload struct {
	Name          string `json:"name"`
	Specification string `json:"specification,omitempty"`
}

type bringBatchUpdateRequest struct {
	Purchase []bringItemPayload `json:"purchase"`
	Recently []bringItemPayload `json:"recently"`
	Remove   []bringItemPayload `json:"remove"`
}

type bringListResponse struct {
	UUID     string             `json:"uuid"`
	Purchase []bringItemPayload `json:"purchase"`
	Recently []bringItemPayload `json:"recently"`
}

// Name returns the Bring! provider identifier.
func (p *BringSyncProvider) Name() string {
	return util.ShoppingListProviderBring
}

func (p *BringSyncProvider) baseURL() string {
	if strings.TrimSpace(p.BaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	}
	return util.DefaultBringAPIBaseURL
}

func (p *BringSyncProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: time.Duration(util.DefaultShoppingListSyncTimeoutSeconds) * time.Second}
}

func formatBringSpecification(item *ShoppingListSyncPayload) string {
	var parts []string
	unit := strings.TrimSpace(item.Unit)
	if item.Quantity > 1 || unit != "" {
		qtyStr := strconv.Itoa(max(1, item.Quantity))
		if unit != "" {
			qtyStr += " " + unit
		}
		parts = append(parts, qtyStr)
	}
	if notes := strings.TrimSpace(item.Notes); notes != "" {
		parts = append(parts, notes)
	}
	return strings.Join(parts, " - ")
}

func (p *BringSyncProvider) doRequest(ctx context.Context, token, method, endpoint string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set(util.RequestHeaderContentType, "application/json")
	}
	return p.client().Do(req)
}

func validateBringIntegration(integration *dbModel.HouseholdIntegration) (string, string, error) {
	if integration == nil || strings.TrimSpace(integration.AccessToken) == "" {
		return "", "", errors.ErrIntegrationMissingCredentials
	}
	listID := strings.TrimSpace(integration.ListID)
	if listID == "" {
		return "", "", errors.ErrIntegrationMissingListID
	}
	return strings.TrimSpace(integration.AccessToken), listID, nil
}

// TestConnection verifies the Bring! credentials and target list UUID.
func (p *BringSyncProvider) TestConnection(ctx context.Context, integration *dbModel.HouseholdIntegration) error {
	token, listID, err := validateBringIntegration(integration)
	if err != nil {
		return err
	}
	endpoint := p.baseURL() + "/bringlists/" + url.PathEscape(listID)
	resp, reqErr := p.doRequest(ctx, token, http.MethodGet, endpoint, nil)
	if reqErr != nil {
		return fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, reqErr)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}
	return nil
}

// Push synchronizes local shopping list items to the configured Bring! list.
func (p *BringSyncProvider) Push(ctx context.Context, integration *dbModel.HouseholdIntegration, items []ShoppingListSyncPayload, refs []dbModel.ShoppingListItemSyncRef) (ShoppingListPushResult, error) {
	token, listID, err := validateBringIntegration(integration)
	if err != nil {
		return ShoppingListPushResult{}, err
	}

	refByItemID := make(map[uint]string, len(refs))
	for i := range refs {
		if refs[i].ShoppingListItemID > 0 {
			refByItemID[refs[i].ShoppingListItemID] = refs[i].ExternalID
		}
	}

	seenItemID := make(map[uint]bool, len(items))
	batch := bringBatchUpdateRequest{
		Purchase: []bringItemPayload{},
		Recently: []bringItemPayload{},
		Remove:   []bringItemPayload{},
	}
	result := ShoppingListPushResult{
		Created: make(map[uint]string),
	}

	queuedEntry := make(map[string]bool, len(items))
	for i := range items {
		item := &items[i]
		seenItemID[item.ID] = true

		if !queuedEntry[item.Name] {
			entry := bringItemPayload{
				Name:          item.Name,
				Specification: formatBringSpecification(item),
			}
			if item.Checked {
				batch.Recently = append(batch.Recently, entry)
			} else {
				batch.Purchase = append(batch.Purchase, entry)
			}
			queuedEntry[item.Name] = true
		}

		if prevExtID, exists := refByItemID[item.ID]; exists {
			if prevExtID != item.Name {
				batch.Remove = append(batch.Remove, bringItemPayload{Name: prevExtID})
				result.Deleted = append(result.Deleted, prevExtID)
				result.Created[item.ID] = item.Name
			} else {
				result.Updated = append(result.Updated, item.ID)
			}
			continue
		}
		result.Created[item.ID] = item.Name
	}

	for i := range refs {
		if refs[i].ShoppingListItemID == 0 || !seenItemID[refs[i].ShoppingListItemID] {
			batch.Remove = append(batch.Remove, bringItemPayload{Name: refs[i].ExternalID})
			result.Deleted = append(result.Deleted, refs[i].ExternalID)
		}
	}

	endpoint := p.baseURL() + "/bringlists/" + url.PathEscape(listID)
	resp, reqErr := p.doRequest(ctx, token, http.MethodPut, endpoint, batch)
	if reqErr != nil {
		return result, fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, reqErr)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("%w: update list status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}

	return result, nil
}

// PullChecked retrieves item checked states from the configured Bring! list.
func (p *BringSyncProvider) PullChecked(ctx context.Context, integration *dbModel.HouseholdIntegration, refs []dbModel.ShoppingListItemSyncRef) (map[string]bool, error) {
	token, listID, err := validateBringIntegration(integration)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return map[string]bool{}, nil
	}

	endpoint := p.baseURL() + "/bringlists/" + url.PathEscape(listID)
	resp, reqErr := p.doRequest(ctx, token, http.MethodGet, endpoint, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, reqErr)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxProviderResponseBytes))
		return nil, fmt.Errorf("%w: get list status %d", errors.ErrIntegrationSyncFailed, resp.StatusCode)
	}

	var listResp bringListResponse
	if err := decodeJSONResponse(resp, &listResp); err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrIntegrationSyncFailed, err)
	}

	inPurchase := make(map[string]bool, len(listResp.Purchase))
	for i := range listResp.Purchase {
		inPurchase[listResp.Purchase[i].Name] = true
	}
	inRecently := make(map[string]bool, len(listResp.Recently))
	for i := range listResp.Recently {
		inRecently[listResp.Recently[i].Name] = true
	}

	checked := make(map[string]bool, len(refs))
	for i := range refs {
		extID := refs[i].ExternalID
		if inPurchase[extID] {
			checked[extID] = false
		} else if inRecently[extID] {
			checked[extID] = true
		}
	}
	return checked, nil
}
