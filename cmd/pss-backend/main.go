package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

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
	dbMigrations, err := migrate.New(config.Migrations, config.Dbm)
	if err != nil {
		logrus.WithError(err).Error("creating migrations")
		return
	}
	if err := dbMigrations.Up(); err != nil && err != migrate.ErrNoChange {
		logrus.WithError(err).Error("doing migrations")
		return
	}

	router := http.NewServeMux()

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
	store := sessions.NewCookieStore(config.SessionKey)
	backend := NewBackend(dbc, store, oauth, log)

	router.Handle("/v1/client/", http.StripPrefix("/v1/client", backend.Handlers()))

	server := http.Server{
		Addr:    config.Listen,
		Handler: router,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
		return
	}
}
