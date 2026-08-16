package organization

import (
	"regexp"
	"strings"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxLabels       = 50
)

var (
	codePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

func normalizePagination(pagination Pagination) (Pagination, error) {
	if pagination.Page == 0 {
		pagination.Page = 1
	}
	if pagination.PageSize == 0 {
		pagination.PageSize = defaultPageSize
	}
	if pagination.Page < 1 {
		return Pagination{}, invalid("page must be at least 1")
	}
	if pagination.PageSize < 1 || pagination.PageSize > maxPageSize {
		return Pagination{}, invalid("page_size must be between 1 and 100")
	}
	return pagination, nil
}

func validateName(name string) error {
	if name == "" {
		return invalid("name is required")
	}
	if len([]rune(name)) > 120 {
		return invalid("name must not exceed 120 characters")
	}
	return nil
}

func validateCode(code string) error {
	if !codePattern.MatchString(code) {
		return invalid("code must contain 1-64 lowercase letters, numbers, or internal hyphens")
	}
	return nil
}

func validateStatus(status string) error {
	if status != StatusActive && status != StatusDisabled {
		return invalid("status must be active or disabled")
	}
	return nil
}

func normalizeIcon(icon, fallback string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return fallback
	}
	if len([]rune(icon)) > 64 {
		return fallback
	}
	return icon
}

func validateID(id, field string) error {
	if !uuidPattern.MatchString(id) {
		return invalid(field + " must be a valid UUID")
	}
	return nil
}

func validateLabels(labels map[string]string) error {
	if len(labels) > maxLabels {
		return invalid("labels must not contain more than 50 entries")
	}
	for key, value := range labels {
		if strings.TrimSpace(key) == "" || len([]rune(key)) > 63 {
			return invalid("label keys must contain 1-63 characters")
		}
		if len([]rune(value)) > 256 {
			return invalid("label values must not exceed 256 characters")
		}
	}
	return nil
}

func cloneLabels(labels map[string]string) map[string]string {
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		copy[key] = value
	}
	return copy
}
