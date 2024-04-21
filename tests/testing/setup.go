package testing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
	"github.com/joho/godotenv"
)

type IntegrationTestEnvironment struct {
	dbname string
	DB     *pgxpool.Pool
}

var TestEnvironment IntegrationTestEnvironment = IntegrationTestEnvironment{}

func getDatabaseUrl(dbname string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("DATABASE_USERNAME"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("DATABASE_HOST"),
		os.Getenv("DATABASE_PORT"),
		dbname,
	)
}

func randomToken(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func findProjectDir() string {
	curDir, _ := os.Getwd()
	projectDir := curDir
	for {
		_, err := os.Stat(projectDir + "/.git")
		if err == nil {
			return projectDir
		}
		projectDir = filepath.Dir(projectDir)
		if projectDir == "/" { // reached root
			return ""
		}
	}
}

func (e *IntegrationTestEnvironment) createDatabase(ctx context.Context) error {
	ic, err := pgx.Connect(context.Background(), getDatabaseUrl("postgres"))
	if err != nil {
		return err
	}
	defer ic.Close(ctx)

	ic.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", e.dbname))
	m, err := migrate.NewMigrator(ctx, ic, "public.schema_version")
	if err != nil {
		return err
	}

	return m.Migrate(ctx)
}

func (e *IntegrationTestEnvironment) initializeDatabase(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, getDatabaseUrl(e.dbname))
	if err != nil {
		return err
	}

	e.DB = pool
	c, err := e.DB.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	m, err := migrate.NewMigrator(ctx, c.Conn(), "public.version_schema")
	if err != nil {
		return err
	}
	m.LoadMigrations(os.DirFS(path.Join(findProjectDir(), "./db/migrations")))
	return m.Migrate(ctx)
}

func (e *IntegrationTestEnvironment) Setup() {
	ctx := context.Background()
	if godotenv.Load(path.Join(findProjectDir(), ".env.dev")) != nil {
		panic("Cannot load .env.test file")
	}

	token, err := randomToken(24)
	if err != nil {
		panic("Cannot generate test database name")
	}

	e.dbname = fmt.Sprintf("test_%s", token)
	err = e.createDatabase(ctx)
	if err != nil {
		e.Cleanup()
		panic(err)
	}

	err = e.initializeDatabase(ctx)
	if err != nil {
		e.Cleanup()
		panic(err)
	}
}

func (e *IntegrationTestEnvironment) Cleanup() {
	c, err := pgx.Connect(context.Background(), getDatabaseUrl("postgres"))
	if err != nil {
		panic(err)
	}

	_, err = c.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH(FORCE)", e.dbname))
	if err != nil {
		panic(fmt.Sprintf("Could not delete test database '%s', because %s", e.dbname, err))
	}
}

func (e *IntegrationTestEnvironment) Reset() {
	_, err := e.DB.Exec(context.Background(), `
		DO
		$do$
		BEGIN
		   EXECUTE
		   (SELECT 'TRUNCATE TABLE ' || string_agg(oid::regclass::text, ', ') || ' CASCADE'
			FROM   pg_class
			WHERE  relkind = 'r'  -- only tables
			AND    relnamespace = 'public'::regnamespace
		   );
		END
		$do$;
	`)

	if err != nil {
		panic(err)
	}
}
