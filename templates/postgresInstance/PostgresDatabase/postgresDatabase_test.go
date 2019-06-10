package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/codesmith-gmbh/cgc/cgccf"
	"github.com/codesmith-gmbh/cgc/cgclog"
	"github.com/codesmith-gmbh/cgc/cgcpg"
	"github.com/codesmith-gmbh/cgc/cgctesting"
	"github.com/jackc/pgx"
	"testing"
)

const (
	dbInstanceIdentifier    = "codesmith"
	dbInstanceStack         = "PostgresInstance-" + dbInstanceIdentifier
	securityGroupOutputName = "SecurityGroup"
	testIngressDescription  = "github.com/codesmith-gmbh/forge/templates/postgresInstance/test"
)

// Before running the tests, you must have a local `codesmith` profile

func TestDatabaseCreationAndDeletion(t *testing.T) {
	defer cgclog.SyncSugaredLogger(log)
	ctx := context.TODO()
	properties := Properties{
		DatabaseName: "test",
	}
	cfg := cgctesting.MustTestConfig()
	p, sgs := mustNewTestProc(cfg)
	//noinspection GoUnhandledErrorResult
	defer sgs.EnsureDescribedIngressRevoked(ctx, p.groupId, testIngressDescription)
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
	config, err := cgcpg.NewConfig(
		cgcpg.RdsIamConfigOption{
			Host: dbInstanceInfo.Host, Port: 5432,
			Database: properties.DatabaseName, Username: properties.DatabaseName,
			Config: cfg,
		},
		cgcpg.TlsConfigOption{Host: dbInstanceInfo.Host},
	)
	if err != nil {
		t.Fatal(err)
	}
	userConn, err := pgx.Connect(config)
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
	defer cgclog.SyncSugaredLogger(log)
	ctx := context.TODO()
	oldProperties := Properties{
		DatabaseName: "test",
	}
	newProperties := Properties{
		DatabaseName: "test2",
	}
	cfg := cgctesting.MustTestConfig()
	p, sgs := mustNewTestProc(cfg)
	//noinspection GoUnhandledErrorResult
	defer sgs.EnsureDescribedIngressRevoked(ctx, p.groupId, testIngressDescription)
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

func mustNewTestProc(config aws.Config) (*proc, *cgcaws.SGS) {
	ctx := context.TODO()
	cf := cloudformation.New(config)
	groupId, err := cgccf.FetchStackOutputValue(ctx, cf, dbInstanceStack, securityGroupOutputName)
	if err != nil {
		panic(err)
	}
	sgs := cgcaws.NewAwsSecurityGroupService(ec2.New(config))
	if err := sgs.OpenSecurityGroup(ctx, groupId, testIngressDescription); err != nil {
		panic(err)
	}
	return newProc(config, dbInstanceIdentifier, groupId), sgs
}
