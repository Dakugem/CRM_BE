package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	// Инициализируем Telegram логгер
	InitTelegramLogger()
	if tgLogger != nil {
		defer tgLogger.Close()
	}

	// Инициализируем rate limiter
	InitRateLimiter()

	if err := OpenDB(); err != nil {
		LogError("Failed to open DB: %v", err)
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer CloseDB()

	if err := os.MkdirAll("./uploads", 0755); err != nil {
		LogWarning("Could not create uploads dir: %v", err)
	}

	mux := http.NewServeMux()

	// Static file serving for uploaded photos
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// ── Auth ────────────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/auth/login", AuthLoginHandler)
	mux.HandleFunc("POST /api/auth/refresh", AuthRefreshHandler)
	mux.HandleFunc("POST /api/auth/logout", RequireAuth(AuthLogoutHandler))

	// ── Tickets ─────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/tickets", RequireAuth(TicketsListHandler))
	mux.HandleFunc("POST /api/tickets", RequireAuth(TicketsCreateHandler))
	mux.HandleFunc("GET /api/tickets/{id}", RequireAuth(TicketsGetHandler))
	mux.HandleFunc("PATCH /api/tickets/{id}", RequireAuth(TicketsUpdateHandler))
	mux.HandleFunc("PATCH /api/tickets/{id}/assign", RequireAuth(TicketsAssignHandler))
	mux.HandleFunc("GET /api/tickets/{id}/comments", RequireAuth(TicketsCommentsListHandler))
	mux.HandleFunc("POST /api/tickets/{id}/comments", RequireAuth(TicketsCommentsCreateHandler))
	mux.HandleFunc("GET /api/tickets/{id}/history", RequireAuth(TicketsHistoryListHandler))
	mux.HandleFunc("POST /api/tickets/{id}/subtasks", RequireAuth(TicketsSubtasksAddHandler))
	mux.HandleFunc("DELETE /api/tickets/{id}/subtasks/{subtaskId}", RequireAuth(TicketsSubtasksRemoveHandler))

	// ── Employees ────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/employees", RequireAuth(EmployeesListHandler))
	mux.HandleFunc("POST /api/employees", RequireAuth(EmployeesCreateHandler))
	mux.HandleFunc("GET /api/employees/{id}", RequireAuth(EmployeesGetHandler))
	mux.HandleFunc("PATCH /api/employees/{id}", RequireAuth(EmployeesUpdateHandler))
	mux.HandleFunc("DELETE /api/employees/{id}", RequireAuth(EmployeesDeleteHandler))
	mux.HandleFunc("PATCH /api/employees/{id}/photo", RequireAuth(EmployeesPhotoHandler))

	// ── Clients ──────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/clients", RequireAuth(ClientsListHandler))
	mux.HandleFunc("POST /api/clients", RequireAuth(ClientsCreateHandler))
	mux.HandleFunc("GET /api/clients/{id}", RequireAuth(ClientsGetHandler))
	mux.HandleFunc("PATCH /api/clients/{id}", RequireAuth(ClientsUpdateHandler))
	mux.HandleFunc("DELETE /api/clients/{id}", RequireAuth(ClientsDeleteHandler))
	mux.HandleFunc("GET /api/clients/{id}/representatives", RequireAuth(ClientsRepsListHandler))
	mux.HandleFunc("POST /api/clients/{id}/representatives", RequireAuth(ClientsRepsCreateHandler))
	mux.HandleFunc("GET /api/clients/{id}/representatives/{repId}", RequireAuth(ClientsRepsGetHandler))
	mux.HandleFunc("PATCH /api/clients/{id}/representatives/{repId}", RequireAuth(ClientsRepsUpdateHandler))
	mux.HandleFunc("DELETE /api/clients/{id}/representatives/{repId}", RequireAuth(ClientsRepsDeleteHandler))

	// ── Sites ────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/sites", RequireAuth(SitesListHandler))
	mux.HandleFunc("POST /api/sites", RequireAuth(SitesCreateHandler))
	mux.HandleFunc("GET /api/sites/{id}", RequireAuth(SitesGetHandler))
	mux.HandleFunc("PATCH /api/sites/{id}", RequireAuth(SitesUpdateHandler))
	mux.HandleFunc("DELETE /api/sites/{id}", RequireAuth(SitesDeleteHandler))
	mux.HandleFunc("GET /api/sites/{id}/equipment", RequireAuth(SitesEquipmentListHandler))
	mux.HandleFunc("POST /api/sites/{id}/equipment", RequireAuth(SitesEquipmentAddHandler))
	mux.HandleFunc("PATCH /api/sites/{id}/equipment/{deviceId}", RequireAuth(SitesEquipmentUpdateHandler))
	mux.HandleFunc("DELETE /api/sites/{id}/equipment/{deviceId}", RequireAuth(SitesEquipmentRemoveHandler))

	// ── Equipment ────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/equipment", RequireAuth(EquipmentListHandler))
	mux.HandleFunc("POST /api/equipment", RequireAuth(EquipmentCreateHandler))
	mux.HandleFunc("GET /api/equipment/{id}", RequireAuth(EquipmentGetHandler))
	mux.HandleFunc("PATCH /api/equipment/{id}", RequireAuth(EquipmentUpdateHandler))
	mux.HandleFunc("DELETE /api/equipment/{id}", RequireAuth(EquipmentDeleteHandler))

	// ── Dashboards ───────────────────────────────────────────────────────────
	// "reorder" must be registered before {id} so the literal wins
	mux.HandleFunc("PATCH /api/dashboards/reorder", RequireAuth(DashboardsReorderHandler))
	mux.HandleFunc("GET /api/dashboards", RequireAuth(DashboardsListHandler))
	mux.HandleFunc("POST /api/dashboards", RequireAuth(DashboardsCreateHandler))
	mux.HandleFunc("GET /api/dashboards/{id}", RequireAuth(DashboardsGetHandler))
	mux.HandleFunc("PATCH /api/dashboards/{id}", RequireAuth(DashboardsUpdateHandler))
	mux.HandleFunc("DELETE /api/dashboards/{id}", RequireAuth(DashboardsDeleteHandler))

	// ── Profile ──────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/profile", RequireAuth(ProfileGetHandler))
	mux.HandleFunc("PATCH /api/profile/photo", RequireAuth(ProfilePhotoHandler))

	// ── References ───────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/references/products", RequireAuth(RefsProductsHandler))
	mux.HandleFunc("GET /api/references/statuses", RefsStatusesHandler)
	mux.HandleFunc("GET /api/references/priorities", RefsPrioritiesHandler)
	mux.HandleFunc("GET /api/references/roles", RequireAuth(RefsRolesHandler))

	LogSuccess("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", rateLimitMiddleware(loggingMiddleware(corsMiddleware(mux)))))
}
