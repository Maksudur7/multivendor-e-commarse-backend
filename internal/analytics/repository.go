package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type AnalyticsOverview struct {
	GMVMTD             float64 `json:"gmv_mtd"`
	ActiveVendors      int     `json:"active_vendors"`
	TotalOrdersMTD     int     `json:"total_orders_mtd"`
	TotalUsers         int     `json:"total_users"`
	PendingOrders      int     `json:"pending_orders"`
	OrderConversionPct string  `json:"order_conversion_pct"`
	Period             string  `json:"period"`
}

func (r *Repository) GetOverview(ctx context.Context) (*AnalyticsOverview, error) {
	if r.db == nil {
		return &AnalyticsOverview{GMVMTD: 0.0, Period: time.Now().Format("January 2006")}, nil
	}
	var totalRevenue float64
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(p.amount),0) FROM payments p
		WHERE p.status = 'COMPLETED' AND p.created_at >= date_trunc('month', now())`).Scan(&totalRevenue)

	var activeVendors int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors WHERE verification_status = 'APPROVED'`).Scan(&activeVendors)

	var totalOrders, totalUsers int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE created_at >= date_trunc('month', now())`).Scan(&totalOrders)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)

	var pendingOrders int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE status = 'PENDING'`).Scan(&pendingOrders)

	conversionPct := 0.0
	if totalUsers > 0 {
		conversionPct = float64(totalOrders) / float64(totalUsers) * 100
	}

	return &AnalyticsOverview{
		GMVMTD:             totalRevenue,
		ActiveVendors:      activeVendors,
		TotalOrdersMTD:    totalOrders,
		TotalUsers:         totalUsers,
		PendingOrders:      pendingOrders,
		OrderConversionPct: fmt.Sprintf("%.2f", conversionPct),
		Period:             time.Now().Format("January 2006"),
	}, nil
}

type DailySale struct {
	Date   string  `json:"date"`
	Orders int     `json:"orders"`
	Revenue float64 `json:"revenue"`
}

type SalesAnalytics struct {
	DailySales   []DailySale `json:"daily_sales"`
	TotalRevenue float64     `json:"total_revenue"`
	TotalOrders  int         `json:"total_orders"`
	PeriodDays   int         `json:"period_days"`
}

func (r *Repository) GetSalesAnalytics(ctx context.Context, days int) (*SalesAnalytics, error) {
	if r.db == nil {
		return &SalesAnalytics{DailySales: []DailySale{}, PeriodDays: days}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT DATE(placed_at) as sale_date, COUNT(*) as order_count, COALESCE(SUM(grand_total),0) as revenue
		FROM master_orders
		WHERE placed_at >= now() - make_interval(days => $1)
		GROUP BY DATE(placed_at)
		ORDER BY sale_date DESC`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dailySales []DailySale
	var totalRevenue float64
	var totalOrders int
	for rows.Next() {
		var date time.Time
		var orderCount int
		var revenue float64
		if err := rows.Scan(&date, &orderCount, &revenue); err == nil {
			totalRevenue += revenue
			totalOrders += orderCount
			dailySales = append(dailySales, DailySale{Date: date.Format("2006-01-02"), Orders: orderCount, Revenue: revenue})
		}
	}
	if dailySales == nil { dailySales = []DailySale{} }

	return &SalesAnalytics{
		DailySales:   dailySales,
		TotalRevenue: totalRevenue,
		TotalOrders:  totalOrders,
		PeriodDays:   days,
	}, nil
}

type CustomerAnalytics struct {
	TotalCustomers     int    `json:"total_customers"`
	RepeatCustomers    int    `json:"repeat_customers"`
	RepeatCustomerRate string `json:"repeat_customer_rate"`
	NewUsersThisMonth  int    `json:"new_users_this_month"`
	TotalUsers         int    `json:"total_users"`
}

func (r *Repository) GetCustomerAnalytics(ctx context.Context) (*CustomerAnalytics, error) {
	if r.db == nil {
		return &CustomerAnalytics{RepeatCustomerRate: "0.00%"}, nil
	}
	var totalCustomers, repeatCustomers int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM master_orders`).Scan(&totalCustomers)
	_ = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT user_id FROM master_orders GROUP BY user_id HAVING COUNT(*) > 1
		) t`).Scan(&repeatCustomers)

	var newUsersThisMonth, totalUsers int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= date_trunc('month', now())`).Scan(&newUsersThisMonth)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)

	repeatRate := 0.0
	if totalCustomers > 0 {
		repeatRate = float64(repeatCustomers) / float64(totalCustomers) * 100
	}

	return &CustomerAnalytics{
		TotalCustomers:     totalCustomers,
		RepeatCustomers:    repeatCustomers,
		RepeatCustomerRate: fmt.Sprintf("%.2f%%", repeatRate),
		NewUsersThisMonth:  newUsersThisMonth,
		TotalUsers:         totalUsers,
	}, nil
}

type ProductAnalyticsItem struct {
	ProductID  string  `json:"product_id"`
	Title      string  `json:"title"`
	Slug       string  `json:"slug"`
	OrderCount int     `json:"order_count"`
	UnitsSold  int     `json:"units_sold"`
	Revenue    float64 `json:"revenue"`
}

type ProductAnalytics struct {
	TopProducts      []ProductAnalyticsItem `json:"top_products"`
	TotalProducts    int                    `json:"total_products"`
	ApprovedProducts int                    `json:"approved_products"`
}

func (r *Repository) GetProductAnalytics(ctx context.Context) (*ProductAnalytics, error) {
	if r.db == nil {
		return &ProductAnalytics{TopProducts: []ProductAnalyticsItem{}}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT p.id::text, p.name, p.slug,
		       COUNT(oi.id) as order_count,
		       COALESCE(SUM(oi.quantity),0) as units_sold,
		       COALESCE(SUM(oi.line_total),0) as revenue
		FROM products p
		LEFT JOIN product_variants pv ON pv.product_id = p.id
		LEFT JOIN order_items oi ON oi.variant_id = pv.id
		GROUP BY p.id, p.name, p.slug
		ORDER BY units_sold DESC
		LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topProducts []ProductAnalyticsItem
	for rows.Next() {
		var item ProductAnalyticsItem
		if err := rows.Scan(&item.ProductID, &item.Title, &item.Slug, &item.OrderCount, &item.UnitsSold, &item.Revenue); err == nil {
			topProducts = append(topProducts, item)
		}
	}
	if topProducts == nil { topProducts = []ProductAnalyticsItem{} }

	var totalProducts, approvedProducts int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&totalProducts)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE approval_status = 'APPROVED'`).Scan(&approvedProducts)

	return &ProductAnalytics{
		TopProducts:      topProducts,
		TotalProducts:    totalProducts,
		ApprovedProducts: approvedProducts,
	}, nil
}

type TrafficAnalytics struct {
	SearchesThisMonth int                      `json:"searches_this_month"`
	ProductPageViews  int                      `json:"product_page_views"`
	TopSearchQueries  []map[string]interface{} `json:"top_search_queries"`
	Period            string                   `json:"period"`
}

func (r *Repository) GetTrafficAnalytics(ctx context.Context) (*TrafficAnalytics, error) {
	if r.db == nil {
		return &TrafficAnalytics{TopSearchQueries: []map[string]interface{}{}, Period: time.Now().Format("January 2006")}, nil
	}
	var totalSearches int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM search_logs WHERE searched_at >= date_trunc('month', now())`).Scan(&totalSearches)
	if err != nil {
		_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM search_logs_2026 WHERE searched_at >= date_trunc('month', now())`).Scan(&totalSearches)
	}

	var productViews int
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(view_count),0) FROM products`).Scan(&productViews)

	var topSearches []map[string]interface{}
	rows, err := r.db.Query(ctx, `
		SELECT query, COUNT(*) as count FROM search_logs_2026
		WHERE searched_at >= now() - INTERVAL '7 days'
		GROUP BY query ORDER BY count DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var q string
			var cnt int
			if err := rows.Scan(&q, &cnt); err == nil {
				topSearches = append(topSearches, map[string]interface{}{"query": q, "count": cnt})
			}
		}
	}
	if topSearches == nil { topSearches = []map[string]interface{}{} }

	return &TrafficAnalytics{
		SearchesThisMonth: totalSearches,
		ProductPageViews:  productViews,
		TopSearchQueries:  topSearches,
		Period:            time.Now().Format("January 2006"),
	}, nil
}

func (r *Repository) QueueCustomReport(ctx context.Context, reportType string, params map[string]interface{}, userID string) (string, error) {
	if r.db == nil {
		return "mock-job-id", nil
	}
	var jobID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO report_jobs (report_type, parameters, status, requested_by)
		VALUES ($1, $2, 'QUEUED', $3::uuid) RETURNING id::text`,
		reportType, params, userID,
	).Scan(&jobID)
	if err != nil {
		return "", err
	}
	return jobID, nil
}
