package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"social-network/pkg/db"
	"social-network/pkg/db/sqlite"
	"social-network/pkg/handlers"
	"social-network/pkg/middleware"
	"social-network/pkg/models/follow"
	"social-network/pkg/models/post"
	"social-network/pkg/sockets/websocket"
)

func main() {
	// Initialize database with WAL mode
	dbPath := "./social-network.db"
	migrationsDir := "./pkg/db/migrations/sqlite"

	// Initialize database (this will run migrations automatically)
	if err := db.Initialize(dbPath, migrationsDir); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// create a new router
	mux := http.NewServeMux()

	// Setup routes
	setupRoutes(mux)

	// Apply CORS middleware
	corsHandler := middleware.CorsMiddleware(mux)

	server := &http.Server{
		Addr:    ":4000",
		Handler: corsHandler,
	}

	// Channel to listen for interrupt or terminate signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Run server in a goroutine
	go func() {
		log.Println("Starting server on :4000...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for signal
	<-stop
	log.Println("Shutting down server...")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// WAL checkpoint before closing DB
	if err := sqlite.WALCheckpoint(db.DB); err != nil {
		log.Printf("Warning: failed to checkpoint WAL before closing: %v", err)
	}

	// Close database
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	log.Println("Server exited gracefully")
}

func setupRoutes(mux *http.ServeMux) {
	// Services initialization
	// POST SERVICE
	postService := post.NewPostService(db.DB)
	postHandler := handlers.NewPostHandler(postService)
	// WebSocket Hub (create first, since FollowService depends on it)
	hub := websocket.NewHub(db.DB)
	go hub.Run()
	// Follow Service (now with hub as second argument)
	followService := follow.NewFollowService(db.DB, hub)
	followHandler := handlers.NewFollowHandler(followService)

	mux.Handle("/ws", middleware.AuthMiddleware(handlers.HandleWebSocket(hub)))

	// Media uploads (to receive media files)
	mediaHandler := handlers.NewMediaHandler()
	mux.HandleFunc("/api/upload/media", mediaHandler.UploadMediaHandler)

	// Serve media files (to display the media)
	mux.Handle("/uploads/media/", http.StripPrefix("/uploads/media/", http.FileServer(http.Dir("./uploads/media/"))))

	// Public routes (no auth required)
	mux.HandleFunc("/api/register", handlers.RegisterHandler)
	mux.HandleFunc("/api/login", handlers.LoginHandler)
	mux.HandleFunc("/api/tenor", handlers.TenorProxyHandler)

	// Development routes
	mux.HandleFunc("/api/dev/clearDB", handlers.DevClearDbHandler)
	mux.HandleFunc("/api/dev/rollback", handlers.DevRollbackHandler)
	mux.HandleFunc("/api/dev/migration-status", handlers.DevMigrationStatusHandler)
	mux.HandleFunc("/api/dev/update-notification-message", handlers.UpdateNotificationMessageHandler)
	mux.Handle("/api/dev/checkAuth", middleware.AuthMiddleware(http.HandlerFunc(handlers.AuthTestHandler)))

	// WAL management endpoints (development only)
	http.HandleFunc("/api/dev/wal-status", handlers.WALStatusHandler)
	http.HandleFunc("/api/dev/wal-checkpoint", handlers.WALCheckpointHandler)

	// Protected routes (auth required)
	mux.Handle("/api/logout", middleware.AuthMiddleware(http.HandlerFunc(handlers.LogoutHandler)))
	mux.Handle("/api/getUser", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetUserByIDHandler)))
	mux.Handle("/api/getUser/batch", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetBatchUsersHandler)))
	mux.Handle("/api/dashboard", middleware.AuthMiddleware(http.HandlerFunc(handlers.DashboardHandler)))
	mux.Handle("/api/edit-profile", middleware.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.EditProfileHandler(w, r, *followService)
	})))
	// -------------------notifications----------------------
	mux.Handle("/api/notifications", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetNotificationsHandler)))
	mux.Handle("/api/notifications/create", middleware.AuthMiddleware(handlers.CreateNotificationHandler(hub)))
	mux.Handle("/api/notifications/read", middleware.AuthMiddleware(http.HandlerFunc(handlers.MarkNotificationAsReadHandler)))
	// -------------------posts----------------------
	mux.Handle("/api/posts", middleware.AuthMiddleware(http.HandlerFunc(postHandler.GetPosts)))
	mux.Handle("/api/posts/user", middleware.AuthMiddleware(http.HandlerFunc(postHandler.GetUserPosts)))
	mux.Handle("/api/post/", middleware.AuthMiddleware(http.HandlerFunc(postHandler.GetPostByID)))
	mux.Handle("/api/create-post", middleware.AuthMiddleware(http.HandlerFunc(postHandler.CreatePost)))
	mux.Handle("/api/edit-post", middleware.AuthMiddleware(http.HandlerFunc(postHandler.EditPost)))
	mux.Handle("/api/delete-post", middleware.AuthMiddleware(http.HandlerFunc(postHandler.DeletePost)))
	mux.Handle("/api/like/post/", middleware.AuthMiddleware(http.HandlerFunc(postHandler.LikePost)))
	mux.Handle("/api/posts/group", middleware.AuthMiddleware(http.HandlerFunc(postHandler.GetGroupPosts)))
	// -------------------follow----------------------
	mux.Handle("/api/unfollow", middleware.AuthMiddleware(http.HandlerFunc(followHandler.UnfollowHandler)))
	mux.Handle("/api/follow/request", middleware.AuthMiddleware(http.HandlerFunc(followHandler.SendFollowRequestHandler)))
	mux.Handle("/api/follow/accept", middleware.AuthMiddleware(http.HandlerFunc(followHandler.AcceptFollowRequestHandler)))
	mux.Handle("/api/follow/reject", middleware.AuthMiddleware(http.HandlerFunc(followHandler.RejectFollowRequestHandler)))
	mux.Handle("/api/follow/pending", middleware.AuthMiddleware(http.HandlerFunc(followHandler.GetPendingRequestsHandler)))
	mux.Handle("/api/user/followers", middleware.AuthMiddleware(http.HandlerFunc(followHandler.GetUserFollowersHandler)))
	mux.Handle("/api/user/following", middleware.AuthMiddleware(http.HandlerFunc(followHandler.GetUserFollowingHandler)))
	// -------------------comment----------------------
	mux.Handle("/api/comment", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetCommentsByPostIDHandler)))
	mux.Handle("/api/comment/create", middleware.AuthMiddleware(http.HandlerFunc(handlers.CommentHandler)))
	mux.Handle("/api/comment/edit", middleware.AuthMiddleware(http.HandlerFunc(handlers.UpdateCommentHandler)))
	mux.Handle("/api/comment/delete", middleware.AuthMiddleware(http.HandlerFunc(handlers.DeleteCommentHandler)))
	mux.Handle("/api/comment/like", middleware.AuthMiddleware(http.HandlerFunc(handlers.LikeCommentHandler)))
	// -------------------group----------------------
	mux.Handle("/api/group", middleware.AuthMiddleware(http.HandlerFunc(handlers.GroupHandler)))
	mux.Handle("/api/group/user", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetUserGroupsHandler)))
	mux.Handle("/api/group/invitation", middleware.AuthMiddleware(handlers.GroupInvitationHandler(hub)))
	mux.Handle("/api/group/request", middleware.AuthMiddleware(handlers.GroupRequestHandler(hub)))
	mux.Handle("/api/group/pending-requests", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetPendingGroupRequestsHandler)))
	mux.Handle("/api/group/accept-invitation", middleware.AuthMiddleware(http.HandlerFunc(handlers.AcceptGroupInvitationHandler(hub))))
	mux.Handle("/api/group/decline-invitation", middleware.AuthMiddleware(http.HandlerFunc(handlers.DeclineGroupInvitationHandler(hub))))
	mux.Handle("/api/group/accept-request", middleware.AuthMiddleware(http.HandlerFunc(handlers.AcceptGroupRequestHandler(hub))))
	mux.Handle("/api/group/decline-request", middleware.AuthMiddleware(http.HandlerFunc(handlers.DeclineGroupRequestHandler(hub))))
	mux.Handle("/api/group/info", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetGroupByIDHandler)))
	mux.Handle("/api/group/members", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetGroupMembersHandler)))
	mux.Handle("/api/group/grant-admin", middleware.AuthMiddleware(http.HandlerFunc(handlers.GrantAdminHandler)))
	mux.Handle("/api/group/revoke-admin", middleware.AuthMiddleware(http.HandlerFunc(handlers.RevokeAdminHandler)))
	mux.Handle("/api/group/grant-creator", middleware.AuthMiddleware(http.HandlerFunc(handlers.GrantCreatorHandler)))
	mux.Handle("/api/group/kick-member", middleware.AuthMiddleware(handlers.KickMemberHandler(hub)))
	mux.Handle("/api/group/edit", middleware.AuthMiddleware(http.HandlerFunc(handlers.EditGroupHandler)))
	mux.Handle("/api/group/join", middleware.AuthMiddleware(http.HandlerFunc(handlers.JoinPublicGroupHandler)))
	mux.Handle("/api/group/leave", middleware.AuthMiddleware(http.HandlerFunc(handlers.LeaveGroupHandler)))
	// -------------------event----------------------
	mux.Handle("/api/event", middleware.AuthMiddleware(handlers.CreateEventHandler(hub)))
	mux.Handle("/api/event/response", middleware.AuthMiddleware(http.HandlerFunc(handlers.CreateEventResponseHandler)))
	mux.Handle("/api/event/group", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetGroupEventsHandler)))
	// -------------------chat----------------------
	mux.Handle("/api/chats", middleware.AuthMiddleware(http.HandlerFunc(handlers.GetUserChatsHandler(hub))))
	mux.Handle("/api/chats/private", middleware.AuthMiddleware(http.HandlerFunc(handlers.CreatePrivateChatHandler)))
	// -------------------search----------------------
	mux.Handle("/api/search/users", middleware.AuthMiddleware(http.HandlerFunc(handlers.SearchUsersHandler)))
	mux.Handle("/api/search/groups", middleware.AuthMiddleware(http.HandlerFunc(handlers.SearchGroupsHandler)))
	mux.Handle("/api/search/posts", middleware.AuthMiddleware(http.HandlerFunc(handlers.SearchPostsHandler)))
	mux.Handle("/api/search", middleware.AuthMiddleware(http.HandlerFunc(handlers.GlobalSearchHandler)))

	// Health check route (pinging the server)
	mux.HandleFunc("/health", handlers.HealthCheckHandler)
}
