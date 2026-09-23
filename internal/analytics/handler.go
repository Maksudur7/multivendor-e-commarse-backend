package analytics

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	a := router.Group("/analytics", authMiddleware)

	// 6 Analytics Endpoints
	a.Get("/overview", h.GetOverview)
	a.Get("/sales", h.GetSalesAnalytics)
	a.Get("/customers", h.GetCustomerAnalytics)
	a.Get("/products", h.GetProductAnalytics)
	a.Get("/traffic", h.GetTrafficAnalytics)
	a.Post("/custom-report", h.GenerateCustomReport)
}

func (h *Handler) GetOverview(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Analytics overview", fiber.Map{
			"gmv_mtd": 0.0, "active_vendors": 0, "active_resellers": 0, "order_conversion_pct": 0.0,
		})
	}
	ctx := c.Context()

	var totalRevenue float64
	h.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(p.amount),0) FROM payments p
		WHERE p.status = 'COMPLETED' AND p.created_at >= date_trunc('month', now())`).Scan(&totalRevenue)

	var activeVendors int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors WHERE verification_status = 'APPROVED'`).Scan(&activeVendors)

	var totalOrders, totalUsers int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE created_at >= date_trunc('month', now())`).Scan(&totalOrders)
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)

	var pendingOrders int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE status = 'PENDING'`).Scan(&pendingOrders)

	conversionPct := 0.0
	if totalUsers > 0 {
		conversionPct = float64(totalOrders) / float64(totalUsers) * 100
	}

	return response.Success(c, fiber.StatusOK, "Analytics overview from NeonDB", fiber.Map{
		"gmv_mtd":               totalRevenue,
		"active_vendors":        activeVendors,
		"total_orders_mtd":      totalOrders,
		"total_users":           totalUsers,
		"pending_orders":        pendingOrders,
		"order_conversion_pct":  fmt.Sprintf("%.2f", conversionPct),
		"period":                time.Now().Format("January 2006"),
	})
}

func (h *Handler) GetSalesAnalytics(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Sales analytics", fiber.Map{"daily_sales": []fiber.Map{}})
	}
	ctx := c.Context()
	days := c.QueryInt("days", 30)
	if days > 90 {
		days = 90
	}

	rows, err := h.db.Query(ctx, `
		SELECT DATE(placed_at) as sale_date, COUNT(*) as order_count, COALESCE(SUM(grand_total),0) as revenue
		FROM master_orders
		WHERE placed_at >= now() - make_interval(days => $1)
		GROUP BY DATE(placed_at)
		ORDER BY sale_date DESC`, days)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load sales data: "+err.Error(), nil)
	}
	defer rows.Close()

	dailySales := []fiber.Map{}
	var totalRevenue float64
	var totalOrders int
	for rows.Next() {
		var date time.Time
		var orderCount int
		var revenue float64
		rows.Scan(&date, &orderCount, &revenue)
		totalRevenue += revenue
		totalOrders += orderCount
		dailySales = append(dailySales, fiber.Map{
			"date": date.Format("2006-01-02"), "orders": orderCount, "revenue": revenue,
		})
	}

	return response.Success(c, fiber.StatusOK, "Sales analytics from NeonDB", fiber.Map{
		"daily_sales":   dailySales,
		"total_revenue": totalRevenue,
		"total_orders":  totalOrders,
		"period_days":   days,
	})
}

func (h *Handler) GetCustomerAnalytics(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Customer analytics", fiber.Map{"repeat_customer_rate": 0.0})
	}
	ctx := c.Context()

	var totalCustomers, repeatCustomers int
	h.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM master_orders`).Scan(&totalCustomers)
	h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT user_id FROM master_orders GROUP BY user_id HAVING COUNT(*) > 1
		) t`).Scan(&repeatCustomers)

	var newUsersThisMonth, totalUsers int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= date_trunc('month', now())`).Scan(&newUsersThisMonth)
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)

	repeatRate := 0.0
	if totalCustomers > 0 {
		repeatRate = float64(repeatCustomers) / float64(totalCustomers) * 100
	}

	return response.Success(c, fiber.StatusOK, "Customer analytics from NeonDB", fiber.Map{
		"total_customers":     totalCustomers,
		"repeat_customers":    repeatCustomers,
		"repeat_customer_rate": fmt.Sprintf("%.2f%%", repeatRate),
		"new_users_this_month": newUsersThisMonth,
		"total_users":          totalUsers,
	})
}

func (h *Handler) GetProductAnalytics(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Product analytics", fiber.Map{"top_products": []fiber.Map{}})
	}
	ctx := c.Context()

	rows, err := h.db.Query(ctx, `
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
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load product analytics: "+err.Error(), nil)
	}
	defer rows.Close()

	topProducts := []fiber.Map{}
	for rows.Next() {
		var id, title, slug string
		var orderCount, unitsSold int
		var revenue float64
		rows.Scan(&id, &title, &slug, &orderCount, &unitsSold, &revenue)
		topProducts = append(topProducts, fiber.Map{
			"product_id": id, "title": title, "slug": slug,
			"order_count": orderCount, "units_sold": unitsSold, "revenue": revenue,
		})
	}

	var totalProducts, approvedProducts int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&totalProducts)
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE approval_status = 'APPROVED'`).Scan(&approvedProducts)

	return response.Success(c, fiber.StatusOK, "Product analytics from NeonDB", fiber.Map{
		"top_products":      topProducts,
		"total_products":    totalProducts,
		"approved_products": approvedProducts,
	})
}

func (h *Handler) GetTrafficAnalytics(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Traffic analytics", fiber.Map{"page_views": 0})
	}
	ctx := c.Context()

	var totalSearches int
	// Try different table name variants for search logs
	err := h.db.QueryRow(ctx, `SELECT COUNT(*) FROM search_logs WHERE searched_at >= date_trunc('month', now())`).Scan(&totalSearches)
	if err != nil {
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM search_logs_2026 WHERE searched_at >= date_trunc('month', now())`).Scan(&totalSearches)
	}

	var productViews int
	h.db.QueryRow(ctx, `SELECT COALESCE(SUM(view_count),0) FROM products`).Scan(&productViews)

	var topSearches []fiber.Map
	rows, err := h.db.Query(ctx, `
		SELECT query, COUNT(*) as count FROM search_logs_2026
		WHERE searched_at >= now() - INTERVAL '7 days'
		GROUP BY query ORDER BY count DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var q string
			var cnt int
			rows.Scan(&q, &cnt)
			topSearches = append(topSearches, fiber.Map{"query": q, "count": cnt})
		}
	}
	if topSearches == nil {
		topSearches = []fiber.Map{}
	}

	return response.Success(c, fiber.StatusOK, "Traffic analytics from NeonDB", fiber.Map{
		"searches_this_month": totalSearches,
		"product_page_views":  productViews,
		"top_search_queries":  topSearches,
		"period":              time.Now().Format("January 2006"),
	})
}

type CustomReportReq struct {
	ReportType string                 `json:"report_type"`
	Parameters map[string]interface{} `json:"parameters"`
}

func (h *Handler) GenerateCustomReport(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	var req CustomReportReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	if req.ReportType == "" {
		return response.ValidationError(c, map[string]string{"report_type": "required"})
	}

	if h.db == nil {
		return response.Created(c, "Report job queued", fiber.Map{"status": "QUEUED", "report_type": req.ReportType})
	}

	ctx := c.Context()
	var jobID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO report_jobs (report_type, parameters, status, requested_by)
		VALUES ($1, $2, 'QUEUED', $3::uuid) RETURNING id::text`,
		req.ReportType, req.Parameters, userID,
	).Scan(&jobID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to queue report: "+err.Error(), nil)
	}

	return response.Created(c, "Custom report job queued in NeonDB", fiber.Map{
		"job_id": jobID, "report_type": req.ReportType, "status": "QUEUED",
		"requested_by": userID, "created_at": time.Now().Format(time.RFC3339),
	})
}
