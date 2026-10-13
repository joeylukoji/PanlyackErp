package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	identity "panlyackerp/internal/modules/identity/adapters/postgres"
	"panlyackerp/internal/platform/db"
)

const demoOrganization = "11111111-1111-4111-8111-111111111111"

func main() {
	databaseURL, email, password := os.Getenv("DATABASE_URL"), os.Getenv("OWNER_EMAIL"), os.Getenv("OWNER_PASSWORD")
	if password == "" {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			log.Fatal("read owner password: ", err)
		}
		password = strings.TrimRight(line, "\r\n")
	}
	if databaseURL == "" || email == "" || password == "" {
		log.Fatal("DATABASE_URL, OWNER_EMAIL and OWNER_PASSWORD are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := (identity.AuthRepository{DB: db.Store{Pool: pool}}).SetOwner(ctx, demoOrganization, email, password); err != nil {
		log.Fatal(err)
	}
	log.Println("owner account ready; previous sessions revoked")
}
