package config

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"strconv"
	"strings"
)

const (
	APP_PORT                 = "APP_PORT"
	APP_HOST                 = "APP_HOST"
	DB_HOST                  = "DB_HOST"
	DB_NAME                  = "DB_NAME"
	DB_USERNAME              = "DB_USERNAME"
	DB_PASS                  = "DB_PASS"
	DB_PORT                  = "DB_PORT"
	DB_CONN_MAX_LIFE_MINUTES = "DB_CONN_MAX_LIFE_MINUTES"
	DB_MAX_OPEN_CONNS        = "DB_MAX_OPEN_CONNS"
	DB_MIN_CONNS             = "DB_MIN_CONNS"
	JAG_DSN                  = "JAG_DSN"
	MAIL_HOST                = "MAIL_HOST"
	MAIL_PORT                = "MAIL_PORT"
	MAIL_USERNAME            = "MAIL_USERNAME"
	MAIL_PASSWORD            = "MAIL_PASSWORD"
	MAIL_COUNT_OF_MAILS      = "MAIL_COUNT_OF_MAILS"
)

type Entity struct {
	App  Application `mapstructure:",squash"`
	DB   Database    `mapstructure:",squash"`
	Jag  Jaeger      `mapstructure:",squash"`
	Mail Mail        `mapstructure:",squash"`
}

func NewConfig() (*Entity, error) {
	readConfigFile, ok := os.LookupEnv("CONFIG_FILE")
	var readFile bool
	var err error
	if ok {
		readFile, err = strconv.ParseBool(readConfigFile)
		if err != nil {
			return nil, fmt.Errorf("NewConfig failed: %w", errors.New("Error during env variable parse"))
		}
	} else {
		readFile = false
	}

	viper.SetConfigFile("./configs/mailboxes.env")

	viper.AllowEmptyEnv(false)
	viper.AutomaticEnv()

	temp := &mailTemp{}
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("NewConfig failed: %w", err)
	}

	err = viper.Unmarshal(temp)
	if err != nil {
		return nil, fmt.Errorf("NewConfig failed: %w", err)
	}

	hosts := strings.Split(temp.Hostname, ",")
	usernames := strings.Split(temp.Username, ",")
	passwords := strings.Split(temp.Password, ",")

	mail := &Mail{
		Hostname:     hosts,
		Port:         temp.Port,
		Username:     usernames,
		Password:     passwords,
		CountOfMails: temp.CountOfMails,
	}

	config := &Entity{Mail: *mail}

	if !readFile {

		config.App = Application{
			Port: viper.GetString(APP_PORT),
			Host: viper.GetString(APP_HOST),
		}

		config.DB = Database{
			Hostname:     viper.GetString(DB_HOST),
			Name:         viper.GetString(DB_NAME),
			User:         viper.GetString(DB_USERNAME),
			Pass:         viper.GetString(DB_PASS),
			Port:         uint16(viper.GetUint32(DB_PORT)),
			ConnLifeTime: viper.GetInt(DB_CONN_MAX_LIFE_MINUTES),
			MaxOpenConns: viper.GetInt32(DB_MAX_OPEN_CONNS),
			MinConns:     viper.GetInt32(DB_MIN_CONNS),
		}

		config.Jag = Jaeger{viper.GetString(JAG_DSN)}

		return config, nil
	}

	viper.SetConfigFile("./configs/.env")

	// Альтернативный метод указания конфиг-файла
	//viper.AddConfigPath(path)
	//viper.SetConfigName("dev")
	//viper.SetConfigType("yml")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// File is absent, but we need to check the ENV Variables, if they absent too throw panic()
		} else {
			return nil, err
		}
	}

	err = viper.Unmarshal(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

type Application struct {
	Port string `mapstructure:"APP_PORT"`
	Host string `mapstructure:"APP_HOST"`
}

type Database struct {
	Hostname     string `mapstructure:"DB_HOST"`
	Name         string `mapstructure:"DB_NAME"`
	User         string `mapstructure:"DB_USERNAME"`
	Pass         string `mapstructure:"DB_PASS"`
	Port         uint16 `mapstructure:"DB_PORT"`
	ConnLifeTime int    `mapstructure:"DB_CONN_MAX_LIFE_MINUTES"`
	MaxOpenConns int32  `mapstructure:"DB_MAX_OPEN_CONNS"`
	MinConns     int32  `mapstructure:"DB_MIN_CONNS"`
}

type Jaeger struct {
	Dsn string `mapstructure:"JAG_DSN"`
}

type Mail struct {
	Hostname     []string
	Port         string
	Username     []string
	Password     []string
	CountOfMails uint32
}

type mailTemp struct {
	Hostname     string `mapstructure:"MAIL_HOST"`
	Port         string `mapstructure:"MAIL_PORT"`
	Username     string `mapstructure:"MAIL_USERNAME"`
	Password     string `mapstructure:"MAIL_PASSWORD"`
	CountOfMails uint32 `mapstructure:"MAIL_COUNT_OF_MAILS"`
}
