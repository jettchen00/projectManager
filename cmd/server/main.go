// Command server 项目管理后台 HTTP 服务入口。
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"projectManager/internal/config"
	httpapi "projectManager/internal/http"
	pmlog "projectManager/internal/log"
	"projectManager/internal/modules/approval"
	"projectManager/internal/modules/changelog"
	"projectManager/internal/modules/formtemplate"
	"projectManager/internal/modules/formvalue"
	"projectManager/internal/modules/owner"
	"projectManager/internal/modules/project"
	"projectManager/internal/modules/snapshot"
	mstore "projectManager/internal/store/mongo"
)

func main() {
	cfg := config.Load()

	// 初始化日志（按配置切换 stdout / 滚动文件，底层基于 zap + lumberjack）。
	pmlog.Init(pmlog.Options{
		Dir:        cfg.Log.Dir,
		MaxSizeMB:  cfg.Log.MaxSizeMB,
		MaxBackups: cfg.Log.MaxBackups,
		Format:     cfg.Log.Format,
		Level:      cfg.Log.Level,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout())
	defer cancel()

	store, err := mstore.Connect(ctx, cfg.Mongo.URI, cfg.Mongo.DB)
	if err != nil {
		pmlog.Errorf("mongo connect err=%v uri=%s db=%s", err, cfg.Mongo.URI, cfg.Mongo.DB)
		os.Exit(1)
	}
	pmlog.Infof("mongo connected db=%s", cfg.Mongo.DB)

	// 仓储
	ownerRepo := owner.NewMongoRepo(store)
	tmplRepo := formtemplate.NewMongoRepo(store)
	projRepo := project.NewMongoRepo(store)
	logRepo := changelog.NewMongoRepo(store)
	formRepo := formvalue.NewMongoRepo(store)
	snRepo := snapshot.NewMongoRepo(store)
	apRepo := approval.NewMongoRepo(store)

	// 服务
	ownerSvc := owner.NewService(ownerRepo)
	tmplSvc := formtemplate.NewService(tmplRepo)
	projSvc := project.NewService(projRepo, ownerSvc)
	logSvc := changelog.NewService(logRepo)
	formSvc := formvalue.NewService(formRepo, tmplSvc, projSvc, logSvc)
	snSvc := snapshot.NewService(snRepo)
	apSvc := approval.NewService(apRepo, projSvc, tmplSvc, formSvc, snSvc)

	// 模板初始化
	if err := tmplSvc.EnsureSeeded(context.Background()); err != nil {
		pmlog.Errorf("tmpl seed err=%v", err)
	}

	deps := &httpapi.Deps{
		Project:   httpapi.NewProjectController(projSvc),
		Form:      httpapi.NewFormController(formSvc, tmplSvc),
		ChangeLog: httpapi.NewChangeLogController(logSvc),
		Approval:  httpapi.NewApprovalController(apSvc),
		Owner:     httpapi.NewOwnerController(ownerSvc),
		Todo:      httpapi.NewTodoController(projSvc),
	}

	webDir := cfg.WebDir
	handler := httpapi.BuildRouter(deps, webDir)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		pmlog.Infof("http listening addr=%s web_dir=%s", cfg.HTTPAddr, webDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			pmlog.Errorf("http listen err=%v", err)
		}
	}()

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	pmlog.Infof("shutting down")
	shCtx, shCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer shCancel()
	if err := srv.Shutdown(shCtx); err != nil {
		pmlog.Errorf("server shutdown err=%v", err)
	}
	if err := store.Close(shCtx); err != nil {
		pmlog.Errorf("mongo close err=%v", err)
	}
}
