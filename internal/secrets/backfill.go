package secrets

import (
	"context"
	"fmt"
)

var legacyColumns = []struct {
	Table   string
	Columns []string
}{
	{"forge_credentials", []string{"access_token"}},
	{"scm_connections", []string{"access_token", "webhook_secret"}},
	{"discord_integrations", []string{"webhook_url", "bot_token"}},
}

func (b *Box) BackfillLegacy(ctx context.Context) (int, error) {
	if b == nil {
		return 0, ErrNotConfigured
	}
	sealed := 0
	for _, t := range legacyColumns {
		for _, col := range t.Columns {
			n, err := b.backfillColumn(ctx, t.Table, col)
			if err != nil {
				return sealed, fmt.Errorf("%s.%s: %w", t.Table, col, err)
			}
			sealed += n
		}
	}
	return sealed, nil
}

func (b *Box) backfillColumn(ctx context.Context, table, column string) (int, error) {
	rows, err := b.pool.Query(ctx, fmt.Sprintf(
		`SELECT id, %s FROM %s WHERE %s <> '' AND %s NOT LIKE $1`, column, table, column, column,
	), sealPrefix+"%")
	if err != nil {
		return 0, err
	}
	type pending struct{ id, value string }
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.value); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, p := range todo {
		enc, err := b.SealString(p.value)
		if err != nil {
			return 0, err
		}
		if _, err := b.pool.Exec(ctx, fmt.Sprintf(
			`UPDATE %s SET %s = $2 WHERE id = $1 AND %s = $3`, table, column, column,
		), p.id, enc, p.value); err != nil {
			return 0, err
		}
	}
	return len(todo), nil
}
