package main

import (
	"context"
	"github.com/Geniuskaa/micro_registration/pkg/config"
	"github.com/Geniuskaa/micro_registration/pkg/database"
	"github.com/Geniuskaa/micro_registration/pkg/server"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"os"
	"time"
)

const (
	service     = "go-app"
	environment = "production"
	id          = 1
)

func main() {

	conf, err := config.NewConfig() // Указать адрес до папки с конфигом
	if err != nil {
		panic("Error with reading config")
	}

	if err := execute(net.JoinHostPort(conf.App.Host, conf.App.Port), conf); err != nil {
		os.Exit(1)
	}

}

func execute(addr string, conf *config.Entity) (err error) {
	ctx, cancel := context.WithCancel(context.Background())

	logger, atom := loggerInit()

	//tp, err := tracerProvider("http://localhost:14268/api/traces") // убрать хардкод, мб на чтение из конфига заменить
	//if err != nil {
	//	panic("Error when setting up tracer")
	//}
	//
	//otel.SetTracerProvider(tp)
	//
	//defer func(ctx context.Context) {
	//	ctx, cancel = context.WithTimeout(ctx, time.Second*5)
	//	defer cancel()
	//	if err := tp.Shutdown(ctx); err != nil {
	//		logger.Error(err)
	//	}
	//}(ctx)
	//
	//tr := tp.Tracer("competition-app")
	//
	//ct, span := tr.Start(ctx, "Go-app")
	//defer span.End()

	pool := database.PoolCreation(ctx, logger, conf) // Panics if something gone wrong

	db := database.NewPostgres(pool)

	//rdb := redis.NewClient(&redis.Options{
	//	Network:             "",
	//	Addr:                "",
	//	Dialer:              nil,
	//	OnConnect:           nil,
	//	Username:            "",
	//	Password:            "",
	//	CredentialsProvider: nil,
	//	DB:                  0,
	//	MaxRetries:          0,
	//	MinRetryBackoff:     0,
	//	MaxRetryBackoff:     0,
	//	DialTimeout:         0,
	//	ReadTimeout:         0,
	//	WriteTimeout:        0,
	//	PoolFIFO:            false,
	//	PoolSize:            0,
	//	PoolTimeout:         0,
	//	MinIdleConns:        0,
	//	MaxIdleConns:        0,
	//	ConnMaxIdleTime:     0,
	//	ConnMaxLifetime:     0,
	//	TLSConfig:           nil,
	//	Limiter:             nil,
	//})

	defer func() {
		cancel()
		pool.Close()
		logger.Sync()
	}()

	//collector := sqlstats.NewStatsCollector(conf.DB.DatabaseName, db.Pool.)
	//collector := pgxpoolprometheus.NewCollector(db.Pool, map[string]string{"db_name": "my_db"})
	reg := prometheus.NewRegistry()
	//reg.MustRegister(collector)

	mux := chi.NewRouter()
	application := server.NewServer(ctx, logger, mux, db, conf) // ct,
	application.Init(atom, reg)

	return application.Start(addr)
}

func loggerInit() (*zap.Logger, zap.AtomicLevel) {

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC1123Z)
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	file, err := os.OpenFile("./logs/logs.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666) // os.Create("./logs/logs.txt")

	if err != nil {
		panic("Error with creating or opening file")
	}

	writeSyncer := zapcore.AddSync(file)
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writeSyncer, atom),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atom),
	)

	logger := zap.New(core)

	return logger, atom
}

func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.Int64("ID", id),
		)),
	)
	return tp, nil
}
