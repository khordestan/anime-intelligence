package postgres

import (
	"context"
	"time"
)

// ViewingHistoryMonth groups completed passes by their recorded UTC archive month.
type ViewingHistoryMonth struct {
	Month         string   `json:"month"`
	Archived      int      `json:"archived"`
	Rated         int      `json:"rated"`
	AverageRating *float64 `json:"averageRating"`
	RatingTotal   int      `json:"-"`
}

func (r *Repository) ViewingHistorySummary(ctx context.Context) ([]ViewingHistoryMonth, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout='2s'"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `WITH bounds AS (
		SELECT statement_timestamp() AS as_of,
			date_trunc('month',statement_timestamp() AT TIME ZONE 'UTC') AS newest
	), history AS (
		SELECT date_trunc('month',w.archived_at AT TIME ZONE 'UTC') AS month,
			count(*) AS archived,count(w.rating) AS rated,avg(w.rating)::float8 AS average,sum(w.rating) AS rating_total
		FROM watch_cycles w CROSS JOIN bounds b
		WHERE w.user_id=$1 AND w.archived_at<=b.as_of
		  AND w.archived_at>=((b.newest-interval '11 months') AT TIME ZONE 'UTC')
		GROUP BY 1
	)
	SELECT to_char(months.start,'YYYY-MM'),COALESCE(h.archived,0),COALESCE(h.rated,0),h.average,COALESCE(h.rating_total,0)
	FROM bounds b
	CROSS JOIN LATERAL generate_series(b.newest-interval '11 months',b.newest,interval '1 month') months(start)
	LEFT JOIN history h ON h.month=months.start
	ORDER BY months.start DESC`, owner.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	months := make([]ViewingHistoryMonth, 0, 12)
	for rows.Next() {
		var month ViewingHistoryMonth
		if err := rows.Scan(&month.Month, &month.Archived, &month.Rated, &month.AverageRating, &month.RatingTotal); err != nil {
			return nil, err
		}
		months = append(months, month)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return months, tx.Commit(ctx)
}
