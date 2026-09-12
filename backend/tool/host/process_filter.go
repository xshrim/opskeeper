package host

import (
	"fmt"
	"strings"
)

// keywordExpression is an OR of AND groups. AND binds tighter than OR, so
// A&B|C means (A and B) or C. Terms are matched as case-insensitive
// substrings of the process search text.
type keywordExpression struct {
	groups [][]string
}

func parseKeywordExpression(raw string) (keywordExpression, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return keywordExpression{}, nil
	}

	var expression keywordExpression
	current := make([]string, 0, 2)
	pendingOperator := byte(0)
	needTerm := true
	for index := 0; index < len(raw); {
		for index < len(raw) && (raw[index] == ' ' || raw[index] == '\t' || raw[index] == '\r' || raw[index] == '\n' || raw[index] == ',') {
			index++
		}
		if index >= len(raw) {
			break
		}
		if raw[index] == '&' || raw[index] == '|' {
			if needTerm {
				return keywordExpression{}, fmt.Errorf("keyword expression cannot start or repeat an operator")
			}
			pendingOperator = raw[index]
			needTerm = true
			index++
			continue
		}

		term, next, err := parseKeywordTerm(raw, index)
		if err != nil {
			return keywordExpression{}, err
		}
		if term == "" {
			return keywordExpression{}, fmt.Errorf("keyword expression contains an empty term")
		}
		if pendingOperator == '|' {
			if len(current) > 0 {
				expression.groups = append(expression.groups, current)
			}
			current = make([]string, 0, 2)
		}
		current = append(current, term)
		pendingOperator = '&'
		needTerm = false
		index = next
	}
	if needTerm {
		return keywordExpression{}, fmt.Errorf("keyword expression cannot end with an operator")
	}
	if len(current) > 0 {
		expression.groups = append(expression.groups, current)
	}
	return expression, nil
}

func parseKeywordTerm(raw string, start int) (string, int, error) {
	if raw[start] == '\'' || raw[start] == '"' {
		quote := raw[start]
		var value strings.Builder
		for index := start + 1; index < len(raw); index++ {
			if raw[index] == '\\' && index+1 < len(raw) {
				index++
				value.WriteByte(raw[index])
				continue
			}
			if raw[index] == quote {
				return strings.TrimSpace(value.String()), index + 1, nil
			}
			value.WriteByte(raw[index])
		}
		return "", len(raw), fmt.Errorf("keyword expression has an unterminated quote")
	}

	startIndex := start
	for start < len(raw) {
		switch raw[start] {
		case ' ', '\t', '\r', '\n', ',', '&', '|':
			return raw[startIndex:start], start, nil
		default:
			start++
		}
	}
	return raw[startIndex:], start, nil
}

func (expression keywordExpression) Match(searchText string) bool {
	if len(expression.groups) == 0 {
		return true
	}
	searchText = strings.ToLower(searchText)
	for _, group := range expression.groups {
		matched := true
		for _, term := range group {
			if !strings.Contains(searchText, strings.ToLower(term)) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func processSearchText(process ProcessInfo) string {
	return process.Name + " " + process.Executable + " " + process.CommandLine
}
