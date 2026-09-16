package repo

import (
	"context"
	"errors"
	"time"

	"inventory-service/global"
	"inventory-service/model"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var (
	// ErrDuplicateStockInboundItem 表示同一张入库单中已经存在该SKU。
	ErrDuplicateStockInboundItem = errors.New("入库单中已存在该SKU")
	// ErrStockInboundNotEditable 表示入库单不存在、无权操作或不是草稿状态。
	ErrStockInboundNotEditable = errors.New("入库单当前不可编辑")
)

// ExistsStockInboundNo 判断入库单号是否已经存在。
func ExistsStockInboundNo(ctx context.Context, inboundNo string) (bool, error) {
	var count int
	if err := global.DB.GetContext(
		ctx,
		&count,
		"SELECT COUNT(*) FROM stock_inbounds WHERE inbound_no = ?",
		inboundNo,
	); err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateStockInbound 创建一张入库单草稿。
func CreateStockInbound(ctx context.Context, inbound *model.StockInbound) error {
	result, err := global.DB.ExecContext(
		ctx,
		`INSERT INTO stock_inbounds (
			inbound_no, supplier_name, status, operator_id, remark
		) VALUES (?, ?, ?, ?, ?)`,
		inbound.InboundNo,
		inbound.SupplierName,
		inbound.Status,
		inbound.OperatorID,
		inbound.Remark,
	)
	if err != nil {
		return err
	}

	inboundID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	inbound.InboundID = inboundID

	return nil
}

// GetStockInboundByID 根据主键查询入库单。
func GetStockInboundByID(ctx context.Context, inboundID int64) (*model.StockInbound, error) {
	var inbound model.StockInbound
	err := global.DB.GetContext(
		ctx,
		&inbound,
		`SELECT
			inbound_id, inbound_no, supplier_name, status, operator_id,
			reviewer_id, submitted_at, reviewed_at, inbound_at,
			remark, review_remark, created_at, updated_at
		FROM stock_inbounds
		WHERE inbound_id = ?`,
		inboundID,
	)
	if err != nil {
		return nil, err
	}

	return &inbound, nil
}

// CreateStockInboundItem 向当前运营人员的草稿入库单添加SKU明细。
func CreateStockInboundItem(ctx context.Context, item *model.StockInboundItem, operatorID int64) error {
	// 查询符合条件的入库单，将查询到的 inbound_id 和传入的 SKU 明细写入明细表
	result, err := global.DB.ExecContext(
		ctx,
		`INSERT INTO stock_inbound_items (inbound_id, sku_id, quantity, cost_price)
		SELECT inbound_id, ?, ?, ?
		FROM stock_inbounds
		WHERE inbound_id = ? AND operator_id = ? AND status = ?`,
		item.SKUID,
		item.Quantity,
		item.CostPrice,
		item.InboundID,
		operatorID,
		model.StockInboundStatusDraft,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError // MySQL 数据库错误的结构体类型
		// 如果这个错误是 MySQL 错误，并且错误码是 1062，说明发生了唯一索引/唯一约束重复冲突，就返回业务错误 ErrDuplicateStockInboundItem
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrDuplicateStockInboundItem
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrStockInboundNotEditable
	}

	itemID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	item.ItemID = itemID

	return nil
}

// GetStockInboundByIDForUpdate 在事务中查询并锁定入库单。
func GetStockInboundByIDForUpdate(ctx context.Context, tx *sqlx.Tx, inboundID int64) (*model.StockInbound, error) {
	var inbound model.StockInbound
	err := tx.GetContext(
		ctx,
		&inbound,
		`SELECT
			inbound_id, inbound_no, supplier_name, status, operator_id,
			reviewer_id, submitted_at, reviewed_at, inbound_at,
			remark, review_remark, created_at, updated_at
		FROM stock_inbounds
		WHERE inbound_id = ?
		FOR UPDATE`,
		inboundID,
	)
	if err != nil {
		return nil, err
	}

	return &inbound, nil
}

// CountStockInboundItems 查询入库单包含的SKU明细数量。
func CountStockInboundItems(ctx context.Context, tx *sqlx.Tx, inboundID int64) (int, error) {
	var count int
	if err := tx.GetContext(
		ctx,
		&count,
		"SELECT COUNT(*) FROM stock_inbound_items WHERE inbound_id = ?",
		inboundID,
	); err != nil {
		return 0, err
	}

	return count, nil
}

// SubmitStockInbound 将草稿入库单更新为待审核状态。
func SubmitStockInbound(ctx context.Context, tx *sqlx.Tx, inboundID int64, submittedAt time.Time) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE stock_inbounds
		SET status = ?, submitted_at = ?
		WHERE inbound_id = ? AND status = ?`,
		model.StockInboundStatusPending,
		submittedAt,
		inboundID,
		model.StockInboundStatusDraft,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	// 如果等于 0，说明没执行sql,防止代码以为提交成功了，但数据库实际上没有修改任何记录
	if rowsAffected == 0 {
		return ErrStockInboundNotEditable
	}

	return nil
}

// ListStockInbounds 查询全部入库申请列表，可按状态筛选。
func ListStockInbounds(ctx context.Context, status *int8, limit int, offset int) ([]model.StockInboundListItem, error) {
	list := make([]model.StockInboundListItem, 0)
	query := `SELECT inbound_id, inbound_no, status, remark, created_at, operator_id
		FROM stock_inbounds
		ORDER BY created_at DESC, inbound_id DESC
		LIMIT ? OFFSET ?`
	args := []interface{}{limit, offset}

	if status != nil {
		query = `SELECT inbound_id, inbound_no, status, remark, created_at, operator_id
			FROM stock_inbounds
			WHERE status = ?
			ORDER BY created_at DESC, inbound_id DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{*status, limit, offset}
	}

	if err := global.DB.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, err
	}

	return list, nil
}

// CountStockInbounds 查询符合条件的全部入库申请总数。
func CountStockInbounds(ctx context.Context, status *int8) (int64, error) {
	var total int64
	query := `SELECT COUNT(*) FROM stock_inbounds`
	args := make([]interface{}, 0)

	if status != nil {
		query = `SELECT COUNT(*) FROM stock_inbounds WHERE status = ?`
		args = []interface{}{*status}
	}

	if err := global.DB.GetContext(ctx, &total, query, args...); err != nil {
		return 0, err
	}

	return total, nil
}

// ListStockInboundItems 查询一张入库单下的全部SKU明细。
func ListStockInboundItems(ctx context.Context, inboundID int64) ([]model.StockInboundItem, error) {
	items := make([]model.StockInboundItem, 0)
	if err := global.DB.SelectContext(
		ctx,
		&items,
		`SELECT item_id, inbound_id, sku_id, quantity, cost_price, created_at, updated_at
		FROM stock_inbound_items
		WHERE inbound_id = ?
		ORDER BY item_id ASC`,
		inboundID,
	); err != nil {
		return nil, err
	}

	return items, nil
}
