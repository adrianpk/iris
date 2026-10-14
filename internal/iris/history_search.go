package iris

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func validateHistorySearch(params historyParams) error {
	if len(params.Text) > 512 || !utf8.ValidString(params.Text) || strings.ContainsRune(params.Text, 0) {
		return problem("invalid_request", "search text must be valid UTF-8 and at most 512 bytes")
	}

	if params.Ordinal < 0 || (params.Ordinal > 0 && len(params.Pair) != 2) {
		return problem("invalid_request", "ordinal requires an explicit pair")
	}

	if len(params.Pair) != 0 && len(params.Pair) != 2 {
		return problem("invalid_request", "pair requires two project selectors")
	}

	return validateHistoryDates(params)
}

func resolveHistoryPair(ctx context.Context, tx *sql.Tx, plan historyPlan) (historyPlan, error) {
	for _, selector := range plan.params.Pair {
		if selector == "" || len(selector) > 4096 {
			return plan, problem("invalid_request", "invalid pair selector")
		}

		value, err := exactProject(ctx, tx, selector)
		if err != nil {
			return plan, err
		}

		plan.pair = append(plan.pair, value.ID)
	}

	slices.Sort(plan.pair)

	if len(plan.pair) == 2 && plan.pair[0] == plan.pair[1] {
		return plan, problem("invalid_request", "pair projects must differ")
	}

	return plan, nil
}

func (plan historyPlan) searchClauses() (clauses []string, args []any) {
	if len(plan.pair) == 2 {
		clauses = append(clauses, "c.project_low=? AND c.project_high=?")
		args = append(args, plan.pair[0], plan.pair[1])
	}

	if plan.params.Ordinal > 0 {
		clauses = append(clauses, "c.pair_ordinal=?")
		args = append(args, plan.params.Ordinal)
	}

	for _, bound := range []struct {
		at     *time.Time
		clause string
	}{{plan.params.Since, "c.created_at>=?"}, {plan.params.Until, "c.created_at<=?"}} {
		if bound.at != nil {
			clauses = append(clauses, bound.clause)
			args = append(args, bound.at.UnixMilli())
		}
	}

	if plan.params.Text != "" {
		clauses = append(clauses, `(instr(c.topic,?)>0 OR EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id=c.id AND instr(m.body,?)>0) OR EXISTS (SELECT 1 FROM participants p WHERE p.conversation_id=c.id AND instr(p.alias_snapshot,?)>0))`)
		args = append(args, plan.params.Text, plan.params.Text, plan.params.Text)
	}

	return clauses, args
}

type historyMatch struct {
	MessageID string `json:"message_id"`
	Sequence  int64  `json:"sequence"`
	Excerpt   string `json:"excerpt"`
}

func readHistoryMatches(ctx context.Context, tx *sql.Tx, id, text string) (matches []historyMatch, truncated bool, resultErr error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,sequence,substr(body,max(1,instr(body,?)-80),512) FROM messages WHERE conversation_id=? AND instr(body,?)>0 ORDER BY sequence LIMIT 6`, text, id, text)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	matches = []historyMatch{}

	for rows.Next() {
		var value historyMatch
		if err := rows.Scan(&value.MessageID, &value.Sequence, &value.Excerpt); err != nil {
			return nil, false, err
		}

		if len(matches) == 5 {
			truncated = true

			break
		}

		matches = append(matches, value)
	}

	return matches, truncated, rows.Err()
}

func validateHistoryDates(params historyParams) error {
	if params.Since != nil && params.Until != nil && params.Since.After(*params.Until) {
		return problem("invalid_request", "since must not follow until")
	}

	return nil
}

func prepareHistoryPair(selectors []string) ([]string, error) {
	result := slices.Clone(selectors)
	for index, selector := range result {
		if selector == "" {
			return nil, problem("invalid_request", "pair selectors must not be empty")
		}

		plan, err := explicitSelection(selector)
		if err != nil {
			return nil, err
		}

		result[index] = plan.selector
	}

	return result, nil
}
