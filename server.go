package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/datatogether/core"
	"github.com/datatogether/sql_datastore"
	"github.com/datatogether/sqlutil"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

var (
	// cfg is the global configuration for the server. It's read in at startup from
	// the config.json file and enviornment variables, see config.go for more info.
	cfg *config
	// log output handled by logrus package
	log = logrus.New()
	// application database connection
	appDB = &sql.DB{}
	// elevate default store
	store = sql_datastore.DefaultStore
)

func init() {
	// configure log
	log.Out = os.Stderr
	log.Level = logrus.InfoLevel
	log.Formatter = &logrus.TextFormatter{
		ForceColors: true,
	}
}

// main app entry point
func main() {
	var err error
	cfg, err = initConfig(os.Getenv("GOLANG_ENV"))
	if err != nil {
		// panic if the server is missing a vital configuration detail
		panic(fmt.Errorf("server configuration error: %s", err.Error()))
	}

	go initPostgres()

	// base server
	s := &http.Server{}
	// connect mux routes to server
	s.Handler = NewServerRoutes()

	// print notable config settings
	printConfigInfo()

	// fire it up!
	log.Infoln("starting server on port", cfg.Port)

	// start server wrapped in a log.Fatal b/c http.ListenAndServe will not
	// return unless there's an error
	log.Fatal(StartServer(cfg, s))
}

// NewServerRoutes returns a Muxer that has all API routes.
// This makes for easy testing using httptest, see server_test.go
func NewServerRoutes() *http.ServeMux {
	m := http.NewServeMux()

	m.Handle("/", middleware(NotFoundHandler))
	m.Handle("/healthcheck", middleware(HealthCheckHandler))

	// serve static content for api documentation
	m.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir(packagePath("docs")))))
	// m.Handle("/javascripts", http.FileServer(http.Dir("public/javascripts")))
	// m.Handle("/stylesheets", http.FileServer(http.Dir("public/stylesheets")))

	m.Handle("/users", middleware(UsersHandler))
	m.Handle("/users/", middleware(UserHandler))

	m.Handle("/primers", middleware(PrimersHandler))
	m.Handle("/primers/", middleware(PrimerHandler))

	m.Handle("/sources", middleware(SourcesHandler))
	m.Handle("/sources/", middleware(SourceHandler))

	m.Handle("/urls", middleware(UrlsHandler))
	m.Handle("/urls/", middleware(UrlHandler))

	// m.Handle("/links", middleware(LinksHandler))
	// m.Handle("/links/", middleware(LinkHandler))

	// m.Handle("/snapshots", middleware(SnapshotsHandler))

	m.Handle("/coverage", middleware(CoverageHandler))

	m.Handle("/repositories", middleware(RepositoriesHandler))
	m.Handle("/repositories/", middleware(RepositoryHandler))

	// m.Handle("/content", middleware())
	// m.Handle("/content/", middleware())

	// m.Handle("/metadata", middleware())
	// m.Handle("/metadata/", middleware())

	m.Handle("/collections", middleware(CollectionsHandler))
	m.Handle("/collections/", middleware(CollectionHandler))

	m.Handle("/uncrawlables", middleware(UncrawlablesHandler))
	m.Handle("/uncrawlables/", middleware(UncrawlableHandler))

	m.Handle("/customcrawls", middleware(CustomCrawlsHandler))
	m.Handle("/customcrawls/", middleware(CustomCrawlHandler))

	m.HandleFunc("/.well-known/acme-challenge/", CertbotHandler)

	return m
}

func initPostgres() {
	log.Infoln("connecting to postgres db")
	if err := sqlutil.ConnectToDb("postgres", cfg.PostgresDbUrl, appDB); err != nil {
		panic(err)
	}
	log.Infoln("connecteded to postgres db")
	created, err := sqlutil.EnsureTables(appDB, packagePath("sql/schema.sql"),
		"primers",
		"sources",
		"urls",
		"links",
		"metadata",
		"snapshots",
		"collections",
		"collection_contents",
		"uncrawlables",
		"archive_requests")
	if err != nil {
		log.Infoln(err)
	}
	if len(created) > 0 {
		log.Infoln("created tables:", created)
	}

	sql_datastore.SetDB(appDB)
	sql_datastore.Register(
		&core.Collection{},
		&core.Link{},
		&core.Primer{},
		&core.Source{},
		&core.Uncrawlable{},
		&core.CustomCrawl{},
		&core.Url{},
	)
}
