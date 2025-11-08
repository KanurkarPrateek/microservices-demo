// Copyright 2024
// Database persistence layer for order data

package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
)

type OrderDatabase struct {
	db *sql.DB
}

func NewOrderDatabase(connectionString string) (*OrderDatabase, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Successfully connected to PostgreSQL database")
	return &OrderDatabase{db: db}, nil
}

func (odb *OrderDatabase) Close() error {
	if odb.db != nil {
		return odb.db.Close()
	}
	return nil
}

func (odb *OrderDatabase) SaveOrder(ctx context.Context, req *pb.PlaceOrderRequest, orderResult *pb.OrderResult, totalAmount *pb.Money) error {
	tx, err := odb.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	orderInsertQuery := `
		INSERT INTO orders (
			order_id, user_id, user_email, user_currency,
			shipping_tracking_id, total_amount_units, total_amount_nanos,
			shipping_cost_units, shipping_cost_nanos,
			shipping_address_street, shipping_address_city,
			shipping_address_state, shipping_address_country,
			shipping_address_zip, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	now := time.Now()
	_, err = tx.ExecContext(ctx, orderInsertQuery,
		orderResult.OrderId,
		req.UserId,
		req.Email,
		req.UserCurrency,
		orderResult.ShippingTrackingId,
		totalAmount.Units,
		totalAmount.Nanos,
		orderResult.ShippingCost.Units,
		orderResult.ShippingCost.Nanos,
		req.Address.StreetAddress,
		req.Address.City,
		req.Address.State,
		req.Address.Country,
		req.Address.ZipCode,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	itemInsertQuery := `
		INSERT INTO order_items (
			order_id, product_id, quantity, cost_units, cost_nanos, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	for _, item := range orderResult.Items {
		_, err = tx.ExecContext(ctx, itemInsertQuery,
			orderResult.OrderId,
			item.Item.ProductId,
			item.Item.Quantity,
			item.Cost.Units,
			item.Cost.Nanos,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Infof("Order %s saved to database successfully", orderResult.OrderId)
	return nil
}

func (odb *OrderDatabase) GetOrder(ctx context.Context, orderID string) (*pb.OrderResult, error) {
	orderQuery := `
		SELECT 
			order_id, shipping_tracking_id,
			shipping_cost_units, shipping_cost_nanos,
			shipping_address_street, shipping_address_city,
			shipping_address_state, shipping_address_country,
			shipping_address_zip
		FROM orders 
		WHERE order_id = $1
	`

	var order pb.OrderResult
	var shippingCost pb.Money
	var address pb.Address

	err := odb.db.QueryRowContext(ctx, orderQuery, orderID).Scan(
		&order.OrderId,
		&order.ShippingTrackingId,
		&shippingCost.Units,
		&shippingCost.Nanos,
		&address.StreetAddress,
		&address.City,
		&address.State,
		&address.Country,
		&address.ZipCode,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	order.ShippingCost = &shippingCost
	order.ShippingAddress = &address

	itemsQuery := `
		SELECT product_id, quantity, cost_units, cost_nanos
		FROM order_items 
		WHERE order_id = $1
	`

	rows, err := odb.db.QueryContext(ctx, itemsQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	var items []*pb.OrderItem
	for rows.Next() {
		var item pb.OrderItem
		var cartItem pb.CartItem
		var cost pb.Money

		err := rows.Scan(
			&cartItem.ProductId,
			&cartItem.Quantity,
			&cost.Units,
			&cost.Nanos,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}

		item.Item = &cartItem
		item.Cost = &cost
		items = append(items, &item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order items: %w", err)
	}

	order.Items = items
	return &order, nil
}

func (odb *OrderDatabase) GetUserOrders(ctx context.Context, userID string) ([]*pb.OrderResult, error) {
	orderQuery := `
		SELECT 
			order_id, shipping_tracking_id,
			shipping_cost_units, shipping_cost_nanos,
			shipping_address_street, shipping_address_city,
			shipping_address_state, shipping_address_country,
			shipping_address_zip
		FROM orders 
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := odb.db.QueryContext(ctx, orderQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*pb.OrderResult
	for rows.Next() {
		var order pb.OrderResult
		var shippingCost pb.Money
		var address pb.Address

		err := rows.Scan(
			&order.OrderId,
			&order.ShippingTrackingId,
			&shippingCost.Units,
			&shippingCost.Nanos,
			&address.StreetAddress,
			&address.City,
			&address.State,
			&address.Country,
			&address.ZipCode,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		order.ShippingCost = &shippingCost
		order.ShippingAddress = &address

		itemsQuery := `
			SELECT product_id, quantity, cost_units, cost_nanos
			FROM order_items 
			WHERE order_id = $1
		`

		itemRows, err := odb.db.QueryContext(ctx, itemsQuery, order.OrderId)
		if err != nil {
			return nil, fmt.Errorf("failed to query order items: %w", err)
		}

		var items []*pb.OrderItem
		for itemRows.Next() {
			var item pb.OrderItem
			var cartItem pb.CartItem
			var cost pb.Money

			err := itemRows.Scan(
				&cartItem.ProductId,
				&cartItem.Quantity,
				&cost.Units,
				&cost.Nanos,
			)
			if err != nil {
				itemRows.Close()
				return nil, fmt.Errorf("failed to scan order item: %w", err)
			}

			item.Item = &cartItem
			item.Cost = &cost
			items = append(items, &item)
		}
		itemRows.Close()

		order.Items = items
		orders = append(orders, &order)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, nil
}
