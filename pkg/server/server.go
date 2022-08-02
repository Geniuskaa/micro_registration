package server

import (
	"context"
	"fmt"
	"github.com/Geniuskaa/micro_registration/pkg/config"
	"github.com/Geniuskaa/micro_registration/pkg/database"
	"github.com/Geniuskaa/micro_registration/pkg/mail"
	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"net/http"
)

type Server struct {
	ctx    context.Context
	logger *zap.Logger
	mux    *chi.Mux
	db     *database.Postgres
	serv   *http.Server
	cfg    *config.Entity
}

func NewServer(ctx context.Context, logger *zap.Logger, mux *chi.Mux, db *database.Postgres, conf *config.Entity) *Server {
	return &Server{ctx: ctx, logger: logger, mux: mux, db: db, cfg: conf}
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}

func (s *Server) Init(atom zap.AtomicLevel, reg *prometheus.Registry) {
	mailServ := mail.NewService(s.cfg, s.logger)

	// Сейчас вызов функции происходит по запросу, в будущем она будет вызываться через сервис (горутина с каналом),
	// который с опр переодичностью будет проверять почту. Также этот сервис будет делать проверки на ошибки.
	// Первые 5 ошибок просто будет попытка переподключения.
	s.mux.Get("/mail", func(writer http.ResponseWriter, request *http.Request) {
		mailServ.CheckMails()
	})
	//serv := user.NewService(s.db, s.logger)
	//
	//s.mux.Mount("/internal", technic.NewHandler(s.ctx, s.logger, atom, reg).Routes())
	//s.mux.Mount("/api/v1/swagger/", httpSwagger.WrapHandler)
	//
	//s.mux.With(metrics.RequestsMetricsMiddleware, authMiddleware.Middleware, s.recoverer).Mount("/api/v1/users", user.NewHandler(s.ctx, s.logger, serv).Routes())
	//s.mux.With(metrics.RequestsMetricsMiddleware, authMiddleware.Middleware, s.recoverer).Mount("/api/v1/devices", device.NewHandler(s.ctx, s.logger, device.NewService(s.logger, s.db)).Routes())

	// Using dynamic change value of countMails need to check in mailbox
	viper.OnConfigChange(func(e fsnotify.Event) {
		s.logger.Info(fmt.Sprintf("Config file changed: %s", e.Name))
		mailServ.ChangeCountOfMailsPerReq(viper.GetUint32("MAIL_COUNT_OF_MAILS"))
	})
	viper.WatchConfig()
}
func (s *Server) Start(addr string) error {
	s.serv = &http.Server{
		Addr:    addr,
		Handler: s,
	}

	s.logger.Info("Service successfully started")
	return s.serv.ListenAndServe()
}

func (s *Server) recoverer(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

		defer func() {
			if err := recover(); err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				writer.Write([]byte("Something going wrong..."))
				s.logger.Error("panic occurred:", zap.Error(fmt.Errorf("", err))) //Подумать как можно подписать получше
			}
		}()
		handler.ServeHTTP(writer, request)
	})
}
