package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/rest"
	"golang.org/x/term"
	"looklook/app/design/internal/config"
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/platform"
	"looklook/app/design/internal/transport"
	"looklook/app/design/migrations"
)

func main() {
	if e := run(); e != nil {
		log.Print(e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: design-server serve|migrate|maintenance|admin create|openapi [-f config.yaml]")
	}
	command := os.Args[1]
	args := os.Args[2:]
	if command == "openapi" {
		a := &transport.API{}
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		return e.Encode(a.OpenAPI())
	}
	if command == "admin" {
		if len(args) == 0 || args[0] != "create" {
			return fmt.Errorf("usage: design-server admin create [-f config.yaml]")
		}
		args = args[1:]
	} else if command != "serve" && command != "migrate" && command != "maintenance" {
		return fmt.Errorf("unknown command: %s", command)
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	file := f.String("f", "app/design/etc/design.yaml", "configuration file")
	if e := f.Parse(args); e != nil {
		return e
	}
	c, e := config.Load(*file)
	if e != nil {
		return e
	}
	db, e := platform.Open(c.MySQLDSN)
	if e != nil {
		return e
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if command == "migrate" {
		if e = migrations.Apply(ctx, db); e != nil {
			return e
		}
		fmt.Println("Design schema migration complete.")
		return nil
	}
	if e = migrations.Check(ctx, db); e != nil {
		return e
	}
	identities := &identity.Service{DB: db, Secret: []byte(c.JWTSecret), Access: time.Duration(c.AccessMinutes) * time.Minute, Refresh: time.Duration(c.RefreshDays) * 24 * time.Hour}
	if command == "admin" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("admin create requires an interactive terminal; passwords are not accepted as command-line arguments")
		}
		fmt.Print("Admin username: ")
		username, e := bufio.NewReader(os.Stdin).ReadString('\n')
		if e != nil {
			return e
		}
		fmt.Print("Password: ")
		password, e := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if e != nil {
			return e
		}
		fmt.Print("Confirm password: ")
		again, e := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if e != nil {
			return e
		}
		if string(password) != string(again) {
			return fmt.Errorf("passwords do not match")
		}
		// Human input time is not part of the DB operation timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		id, e := identities.CreateAdmin(ctx, 0, strings.TrimSpace(username), string(password), nil, true, "bootstrap")
		if e != nil {
			return e
		}
		fmt.Printf("Created initial administrator %s.\n", id)
		return nil
	}
	if e = os.MkdirAll(c.StorageDir, 0700); e != nil {
		return e
	}
	api := transport.New(c, identities, &catalog.Service{DB: db})
	if command == "maintenance" {
		return api.Maintenance(ctx)
	}
	if e = api.Ready(ctx); e != nil {
		return e
	}
	srv, e := rest.NewServer(c.RestConf, rest.WithNotFoundHandler(api.NotFoundHandler()), rest.WithNotAllowedHandler(api.ErrorHandler(405, "请求方法不允许")))
	if e != nil {
		return e
	}
	defer srv.Stop()
	srv.AddRoutes(api.RestRoutes())
	log.Printf("Design API listening on %s:%d", c.Host, c.Port)
	background, stop := context.WithCancel(context.Background())
	defer stop()
	go api.RunMaintenance(background)
	go api.Tasks.Run(background)
	srv.Start()
	return nil
}
