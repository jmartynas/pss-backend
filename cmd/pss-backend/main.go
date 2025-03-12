package main

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/boj/redistore"
	"github.com/bxcodec/dbresolver/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gorilla/sessions"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var config struct {
	Listen string `envconfig:"listen"`

	Dbm        string   `envconfig:"dbm"`
	Dbs        []string `envconfig:"dbs"`
	Migrations string   `envconfig:"migrations"`

	Production bool `envconfig:"production"`

	SessionKey []byte `envconfig:"session_key"`

	Google struct {
		ClientID     string `envconfig:"client_id"`
		ClientSecret string `envconfig:"client_secret"`
		RedirectURL  string `envconfig:"redirect_url"`
	}

	RedisURL string `envconfig:"redis_url"`
}

func main() {
	if err := envconfig.Process("PSS", &config); err != nil {
		logrus.WithError(err).Error("parsing environment variables")
		return
	}
	log := logrus.New()
	if config.Production {
		file, err := os.OpenFile("pss_backend.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Failed to open log file:", err)
		}

		log.SetOutput(file)
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetOutput(os.Stdout)
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// mysql master connection
	master, err := sql.Open("mysql", config.Dbm)
	if err != nil {
		log.WithError(err).Error("connecting to master DB")
		return
	}
	if err := master.Ping(); err != nil {
		log.WithError(err).Error("could not ping master database")
		return
	}
	conns := []dbresolver.OptionFunc{
		dbresolver.WithPrimaryDBs(master),
	}

	// mysql slave connections
	for _, slaveStr := range config.Dbs {
		slave, err := sql.Open("mysql", slaveStr)
		if err != nil {
			log.WithError(err).Error("connecting to slave DBs")
			return
		}
		conns = append(conns, dbresolver.WithReplicaDBs(slave))
	}

	// all mysql connections
	dbc := dbresolver.New(conns...)
	defer dbc.Close()

	// migrations
	dbMigrations, err := migrate.New(config.Migrations, "mysql://"+config.Dbm)
	if err != nil {
		logrus.WithError(err).Error("creating migrations")
		return
	}
	if err := dbMigrations.Up(); err != nil && err != migrate.ErrNoChange {
		logrus.WithError(err).Error("doing migrations")
		return
	}

	// oauth2 config
	var oauth *oauth2.Config
	if config.Google.ClientID != "" &&
		config.Google.ClientSecret != "" &&
		config.Google.RedirectURL != "" {
		oauth = &oauth2.Config{
			ClientID:     config.Google.ClientID,
			ClientSecret: config.Google.ClientSecret,
			RedirectURL:  config.Google.RedirectURL,
			Scopes:       []string{"email"},
			Endpoint:     google.Endpoint,
		}
	}

	var store sessions.Store
	if config.RedisURL != "" {
		redisStore, err := redistore.NewRediStoreWithURL(10, config.RedisURL, config.SessionKey)
		if err != nil {
			logrus.WithError(err).Error("creating redis session store")
			return
		}
		defer redisStore.Close()
		store = redisStore
	} else {
		store = sessions.NewCookieStore(config.SessionKey)
	}
	backend := NewBackend(dbc, store, oauth, log)

	router := http.NewServeMux()
	router.Handle("/v1/client/", http.StripPrefix("/v1/client", backend.Handlers()))

	server := http.Server{
		Addr:    config.Listen,
		Handler: router,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Error("listening and serving")
		return
	}
}
