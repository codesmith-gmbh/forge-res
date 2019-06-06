package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/jackc/pgx"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Before running the tests, you must have a local `codesmith` profile

func TestDatabaseCreationAndDeletion(t *testing.T) {
	defer SyncSugaredLogger(log)
	ctx := context.TODO()
	properties := Properties{
		DatabaseName: "test",
	}
	cfg := MustConfig(external.WithSharedConfigProfile("codesmith"))
	p := newProc(cfg)
	adminConn, err := p.connect(ctx, properties, postgrepDatabase)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := adminConn.Close(); err != nil {
			t.Error(err)
		}
	}()
	// 1. create the database and the admin user (same name as database, login via IAM role)
	err = createDatabase(adminConn, properties)
	if err != nil {
		t.Fatal(err)
	}
	// 2. when the test is over, we delete the user and drop the database.
	defer
		func() {
			if err := deleteDatabase(adminConn, properties); err != nil {
				t.Error(err)
			}
		}()
	// 3. We test by logging in with the user and creating a simple table.
	dbInstanceInfo, err := p.dbInstanceInfo(ctx, properties)
	if err != nil {
		t.Fatal(err)
	}
	password, err := BuildAuthToken(dbInstanceInfo.Host, p.smg.Region, properties.DatabaseName, p.smg.Credentials)
	if err != nil {
		t.Fatal(err)
	}
	tlsConfig, err := loadTlsConfig(dbInstanceInfo.Host)
	if err != nil {
		t.Fatal(err)
	}
	userConnConfig := pgx.ConnConfig{
		Host:      dbInstanceInfo.Host,
		Port:      dbInstanceInfo.Port,
		User:      properties.DatabaseName,
		Database:  properties.DatabaseName,
		Password:  password,
		TLSConfig: tlsConfig,
	}
	userConn, err := pgx.Connect(userConnConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := userConn.Close(); err != nil {
			t.Error(err)
		}
	}()
	tag, err := userConn.Exec("create table test (id int primary key)")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "CREATE TABLE" {
		t.Errorf("expected CREATE TABLE got %s", tag)
	}
}

func TestDatabaseRename(t *testing.T) {
	defer SyncSugaredLogger(log)
	ctx := context.TODO()
	oldProperties := Properties{
		DatabaseName: "test",
	}
	newProperties := Properties{
		DatabaseName: "test2",
	}
	cfg := MustConfig(external.WithSharedConfigProfile("codesmith"))
	p := newProc(cfg)
	adminConn, err := p.connect(ctx, oldProperties, postgrepDatabase)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := adminConn.Close(); err != nil {
			t.Error(err)
		}
	}()
	// 1. create the database and the admin user (same name as database, login via IAM role)
	err = createDatabase(adminConn, oldProperties)
	if err != nil {
		t.Fatal(err)
	}
	renameSuccessful := aws.Bool(true)
	// 2. when the test is over, we delete the user and drop the database.
	defer
		func() {
			var properties Properties
			if *renameSuccessful {
				properties = newProperties
			} else {
				properties = oldProperties
			}
			if err := deleteDatabase(adminConn, properties); err != nil {
				t.Error(err)
			}
		}()
	// 3. We rename the database
	err = updateDatabase(adminConn, oldProperties, newProperties)
	if err != nil {
		*renameSuccessful = false
		t.Fatal(err)
	}
}

func BuildAuthToken(endpoint, region, dbUser string, credProvider aws.CredentialsProvider) (string, error) {
	// the scheme is arbitrary and is only needed because validation of the URL requires one.
	if !(strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")) {
		endpoint = "https://" + endpoint + ":5432"
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	values := req.URL.Query()
	values.Set("Action", "connect")
	values.Set("DBUser", dbUser)
	req.URL.RawQuery = values.Encode()

	signer := v4.Signer{
		Credentials: credProvider,
	}
	_, err = signer.Presign(req, nil, "rds-db", region, 15*time.Minute, time.Now())
	if err != nil {
		return "", err
	}

	url := req.URL.String()
	if strings.HasPrefix(url, "http://") {
		url = url[len("http://"):]
	} else if strings.HasPrefix(url, "https://") {
		url = url[len("https://"):]
	}

	return url, nil
}
