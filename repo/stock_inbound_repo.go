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
	// ErrStockInboundItemUpdateFailed 表示已确认明细存在，但更新没有影响任何行。
	ErrStockInboundItemUpdateFailed = errors.New("入库明细更新异常")
	// ErrStockInboundItemDeleteFailed 表示已确认明细存在，但删除没有影响任何行。
	ErrStockInboundItemDeleteFailed = errors.New("入库明细删除异常")
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
			reviewer_id, submitted_at, reviewed_at,
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

// GetStockInboundItemByIDForUpdate 在事务中查询并锁定指定入库单下的一条明细。
func GetStockInboundItemByIDForUpdate(ctx context.Context, tx *sqlx.Tx, inboundID int64, itemID int64) (*model.StockInboundItem, error) {
	var item model.StockInboundItem
	err := tx.GetContext(
		ctx,
		&item,
		`SELECT item_id, inbound_id, sku_id, quantity, cost_price, created_at, updated_at
		FROM stock_inbound_items
		WHERE inbound_id = ? AND item_id = ?
		FOR UPDATE`,
		inboundID, itemID,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateStockInboundItem 更新一条草稿SKU明细的数量和成本价。
func UpdateStockInboundItem(ctx context.Context, tx *sqlx.Tx, inboundID int64, itemID int64, quantity int, costPrice int64) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE stock_inbound_items
		SET quantity = ?, cost_price = ?
		WHERE inbound_id = ? AND item_id = ?`,
		quantity, costPrice, inboundID, itemID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrStockInboundItemUpdateFailed
	}
	return nil
}

// DeleteStockInboundItem 删除指定入库单中的一条SKU明细。
func DeleteStockInboundItem(ctx context.Context, tx *sqlx.Tx, inboundID int64, itemID int64) error {
	result, err := tx.ExecContext(
		ctx,
		"DELETE FROM stock_inbound_items WHERE inbound_id = ? AND item_id = ?",
		inboundID, itemID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrStockInboundItemDeleteFailed
	}
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
			reviewer_id, submitted_at, reviewed_at,
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

// CancelStockInbound 将创建人的草稿或待审核入库单标记为已取消。
func CancelStockInbound(ctx context.Context, tx *sqlx.Tx, inboundID int64, operatorID int64) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE stock_inbounds
		SET status = ?
		WHERE inbound_id = ? AND operator_id = ? AND status IN (?, ?)`,
		model.StockInboundStatusCancelled,
		inboundID, operatorID,
		model.StockInboundStatusDraft, model.StockInboundStatusPending,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
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

// ListStockInboundItemsTx 在审核事务中读取整张入库单的明细。
func ListStockInboundItemsTx(ctx context.Context, tx *sqlx.Tx, inboundID int64) ([]model.StockInboundItem, error) {
	items := make([]model.StockInboundItem, 0)
	if err := tx.SelectContext(
		ctx,
		&items,
		`SELECT item_id, inbound_id, sku_id, quantity, cost_price, created_at, updated_at
		FROM stock_inbound_items
		WHERE inbound_id = ?
		ORDER BY sku_id ASC`,
		inboundID,
	); err != nil {
		return nil, err
	}
	return items, nil
}

// IncreaseSKUAvailableStock 新建库存记录或原子增加已有SKU的可用库存。
func IncreaseSKUAvailableStock(ctx context.Context, tx *sqlx.Tx, skuID int64, quantity int) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO sku_stocks (sku_id, available_stock, locked_stock)
		VALUES (?, ?, 0)
		ON DUPLICATE KEY UPDATE available_stock = available_stock + ?`,
		skuID, quantity, quantity,
	)
	return err
}

// CreateStockInboundLog 写入一条SKU审核入库流水。
func CreateStockInboundLog(ctx context.Context, tx *sqlx.Tx, inboundNo string, skuID int64, quantity int, reviewerID int64) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO stock_logs (
			sku_id, business_type, business_no, available_change, locked_change, operator_id
		) VALUES (?, ?, ?, ?, 0, ?)`,
		skuID, model.StockBusinessTypeInbound, inboundNo, quantity, reviewerID,
	)
	return err
}

// ApproveStockInbound 将待审核入库单标记为已入库。
func ApproveStockInbound(ctx context.Context, tx *sqlx.Tx, inboundID int64, reviewerID int64, reviewedAt time.Time) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE stock_inbounds
		SET status = ?, reviewer_id = ?, reviewed_at = ?
		WHERE inbound_id = ? AND status = ?`,
		model.StockInboundStatusCompleted, reviewerID, reviewedAt,
		inboundID, model.StockInboundStatusPending,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrStockInboundNotEditable
	}
	return nil
}

// RejectStockInbound 将待审核入库单标记为已拒绝并保存拒绝原因。
func RejectStockInbound(ctx context.Context, tx *sqlx.Tx, inboundID int64, reviewerID int64, reviewedAt time.Time, reviewRemark string) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE stock_inbounds
		SET status = ?, reviewer_id = ?, reviewed_at = ?, review_remark = ?
		WHERE inbound_id = ? AND status = ?`,
		model.StockInboundStatusRejected, reviewerID, reviewedAt, reviewRemark,
		inboundID, model.StockInboundStatusPending,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrStockInboundNotEditable
	}
	return nil
}
