package postgres

import (
	"context"
	"time"
)

type AnalysisBudgetResult int

const (
	AnalysisBudgetGranted AnalysisBudgetResult = iota + 1
	AnalysisOwnerLimit
	AnalysisDeploymentLimit
)

// ReserveAnalysisCall atomically takes one owner and deployment allowance.
// Replicas that share PostgreSQL also share these fixed one-minute windows.
func (r *Repository) ReserveAnalysisCall(ctx context.Context) (AnalysisBudgetResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)

	var window time.Time
	if err := tx.QueryRow(ctx, `SELECT date_trunc('minute', statement_timestamp())`).Scan(&window); err != nil {
		return 0, err
	}
	global, err := tx.Exec(ctx, `UPDATE catalog_analysis_global_budget
		SET window_start=$1, requests=CASE WHEN window_start<$1 THEN 1 ELSE requests+1 END
		WHERE singleton AND (window_start<$1 OR (window_start=$1 AND requests<12))`, window)
	if err != nil {
		return 0, err
	}
	if global.RowsAffected() == 0 {
		return AnalysisDeploymentLimit, nil
	}
	owned, err := tx.Exec(ctx, `INSERT INTO catalog_analysis_owner_budget(user_id,window_start,requests)
		VALUES($1,$2,1)
		ON CONFLICT (user_id) DO UPDATE SET window_start=excluded.window_start,
		  requests=CASE WHEN catalog_analysis_owner_budget.window_start<excluded.window_start
		                THEN 1 ELSE catalog_analysis_owner_budget.requests+1 END
		WHERE catalog_analysis_owner_budget.window_start<excluded.window_start
		   OR (catalog_analysis_owner_budget.window_start=excluded.window_start
		       AND catalog_analysis_owner_budget.requests<3)`, owner.ID, window)
	if err != nil {
		return 0, err
	}
	if owned.RowsAffected() == 0 {
		return AnalysisOwnerLimit, nil
	}
	return AnalysisBudgetGranted, tx.Commit(ctx)
}
