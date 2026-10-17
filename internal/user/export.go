package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ExportData holds the full LGPD data export for a user.
type ExportData struct {
	User              exportUser               `json:"user"`
	Transactions      []exportTransaction      `json:"transactions"`
	RecurringExpenses []exportRecurringExpense `json:"recurring_expenses"`
	MonthlySavings    []exportMonthlySaving    `json:"monthly_savings"`
	AuditLog          []exportAuditEntry       `json:"audit_log"`
}

type exportUser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type exportTransaction struct {
	ID          string    `json:"id"`
	Amount      string    `json:"amount"`
	Direction   string    `json:"direction"`
	Description *string   `json:"description"`
	CategoryID  *string   `json:"category_id"`
	IsShared    bool      `json:"is_shared"`
	ClientID    string    `json:"client_id"`
	Date        string    `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

type exportRecurringExpense struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Amount      string    `json:"amount"`
	DueDay      int       `json:"due_day"`
	CategoryID  *string   `json:"category_id"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type exportMonthlySaving struct {
	ID          string `json:"id"`
	Year        int    `json:"year"`
	Month       int    `json:"month"`
	GoalAmount  string `json:"goal_amount"`
	SavedAmount string `json:"saved_amount"`
}

type exportAuditEntry struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

func (r *Repository) buildExport(ctx context.Context, userID string) (ExportData, error) {
	var export ExportData

	if err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, created_at FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&export.User.ID, &export.User.Name, &export.User.Email, &export.User.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return ExportData{}, ErrNotFound
		}
		return ExportData{}, fmt.Errorf("user.repository.buildExport: user: %w", err)
	}

	txRows, err := r.db.QueryContext(ctx, `
		SELECT id, amount::text, direction, description, category_id, is_shared, client_id, date, created_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY date DESC, created_at DESC`, userID)
	if err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: transactions: %w", err)
	}
	export.Transactions = []exportTransaction{}
	for txRows.Next() {
		var t exportTransaction
		var catID sql.NullString
		var date time.Time
		if err := txRows.Scan(&t.ID, &t.Amount, &t.Direction, &t.Description,
			&catID, &t.IsShared, &t.ClientID, &date, &t.CreatedAt); err != nil {
			_ = txRows.Close()
			return ExportData{}, fmt.Errorf("user.repository.buildExport: transactions scan: %w", err)
		}
		if catID.Valid {
			t.CategoryID = &catID.String
		}
		t.Date = date.Format("2006-01-02")
		export.Transactions = append(export.Transactions, t)
	}
	if err := txRows.Close(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: transactions close: %w", err)
	}
	if err := txRows.Err(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: transactions rows: %w", err)
	}

	recRows, err := r.db.QueryContext(ctx, `
		SELECT id, name, amount::text, due_day, category_id, active, created_at
		FROM recurring_expenses
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: recurring_expenses: %w", err)
	}
	export.RecurringExpenses = []exportRecurringExpense{}
	for recRows.Next() {
		var re exportRecurringExpense
		var catID sql.NullString
		if err := recRows.Scan(&re.ID, &re.Description, &re.Amount,
			&re.DueDay, &catID, &re.Active, &re.CreatedAt); err != nil {
			_ = recRows.Close()
			return ExportData{}, fmt.Errorf("user.repository.buildExport: recurring_expenses scan: %w", err)
		}
		if catID.Valid {
			re.CategoryID = &catID.String
		}
		export.RecurringExpenses = append(export.RecurringExpenses, re)
	}
	if err := recRows.Close(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: recurring_expenses close: %w", err)
	}
	if err := recRows.Err(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: recurring_expenses rows: %w", err)
	}

	savRows, err := r.db.QueryContext(ctx, `
		SELECT id, year, month, goal_amount::text, saved_amount::text
		FROM monthly_savings
		WHERE user_id = $1
		ORDER BY year DESC, month DESC`, userID)
	if err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: monthly_savings: %w", err)
	}
	export.MonthlySavings = []exportMonthlySaving{}
	for savRows.Next() {
		var ms exportMonthlySaving
		if err := savRows.Scan(&ms.ID, &ms.Year, &ms.Month, &ms.GoalAmount, &ms.SavedAmount); err != nil {
			_ = savRows.Close()
			return ExportData{}, fmt.Errorf("user.repository.buildExport: monthly_savings scan: %w", err)
		}
		export.MonthlySavings = append(export.MonthlySavings, ms)
	}
	if err := savRows.Close(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: monthly_savings close: %w", err)
	}
	if err := savRows.Err(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: monthly_savings rows: %w", err)
	}

	auditRows, err := r.db.QueryContext(ctx, `
		SELECT id, action, metadata, created_at
		FROM audit_log
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: audit_log: %w", err)
	}
	export.AuditLog = []exportAuditEntry{}
	for auditRows.Next() {
		var a exportAuditEntry
		var meta []byte
		if err := auditRows.Scan(&a.ID, &a.Action, &meta, &a.CreatedAt); err != nil {
			_ = auditRows.Close()
			return ExportData{}, fmt.Errorf("user.repository.buildExport: audit_log scan: %w", err)
		}
		a.Metadata = json.RawMessage(meta)
		export.AuditLog = append(export.AuditLog, a)
	}
	if err := auditRows.Close(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: audit_log close: %w", err)
	}
	if err := auditRows.Err(); err != nil {
		return ExportData{}, fmt.Errorf("user.repository.buildExport: audit_log rows: %w", err)
	}

	return export, nil
}

func (r *Repository) insertAuditLog(ctx context.Context, userID, action, ip string) error {
	meta := fmt.Sprintf(`{"ip":%q}`, ip)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, action, entity, entity_id, metadata) VALUES ($1, $2, 'user', $1, $3)`,
		userID, action, meta,
	)
	if err != nil {
		return fmt.Errorf("user.repository.insertAuditLog: %w", err)
	}
	return nil
}
