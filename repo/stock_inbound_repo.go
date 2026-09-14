package repo

import (
	"context"

	"inventory-service/global"
	"inventory-service/model"

	"github.com/jmoiron/sqlx"
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

// CreateStockInbounds 在同一个事务中写入一张入库单的全部 SKU 明细。
func CreateStockInbounds(ctx context.Context, tx *sqlx.Tx, inbounds []*model.StockInbound) error {
	stmt, err := tx.PreparexContext(
		ctx,
		`INSERT INTO stock_inbounds (
			inbound_no, supplier_name, sku_id, quantity, cost_price,
			status, operator_id, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, inbound := range inbounds {
		result, err := stmt.ExecContext(
			ctx,
			inbound.InboundNo,
			inbound.SupplierName,
			inbound.SKUID,
			inbound.Quantity,
			inbound.CostPrice,
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
	}

	return nil
}
