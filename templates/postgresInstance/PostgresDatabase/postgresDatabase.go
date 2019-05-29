package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/jackc/pgx"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	"os"
	"path"
)

const (
	postgrepDatabase = "postgres"
)

var (
	log                  = MustSugaredLogger()
	dbInstanceIdentifier = os.Getenv("DB_INSTANCE_IDENTIFIER")
	secretName           = "codesmith-forge--rds--" + dbInstanceIdentifier
)

type proc struct {
	smg *secretsmanager.SecretsManager
}

func newProc(cfg aws.Config) *proc {
	return &proc{
		smg: secretsmanager.New(cfg),
	}
}

func main() {
	defer SyncSugaredLogger(log)
	log.Debugw("", "dbInstanceIdentifier", dbInstanceIdentifier)
	if dbInstanceIdentifier == "" {
		panic("DB_INSTANCE_IDENTIFIER is not defined")
	}
	cfg := MustConfig()
	p := newProc(cfg)
	lambda.Start(cfn.LambdaWrap(p.processEvent))
}

type Properties struct {
	DatabaseName string
}

func postgresDatabaseProperties(input map[string]interface{}) (Properties, error) {
	var properties Properties
	if err := mapstructure.Decode(input, &properties); err != nil {
		return properties, err
	}
	if properties.DatabaseName == "" {
		return properties, errors.New("database name must be defined")
	}

	return properties, nil
}

func (p *proc) processEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
	log.Debugw("processing event", "event", event)
	properties, err := postgresDatabaseProperties(event.ResourceProperties)
	if err != nil {
		return "", nil, err
	}
	conn, err := p.connect(ctx, properties, postgrepDatabase)
	if err != nil {
		return "", nil, err
	}

	switch event.RequestType {
	case cfn.RequestDelete:
		if err := deleteDatabase(conn, properties); err != nil {
			return "", nil, err
		}
	case cfn.RequestCreate:
		if err := createDatabase(conn, properties); err != nil {
			return "", nil, err
		}
	case cfn.RequestUpdate:
		oldProperties, err := postgresDatabaseProperties(event.OldResourceProperties)
		if err != nil {
			return event.LogicalResourceID, nil, err
		}
		if err := updateDatabase(conn, oldProperties, properties); err != nil {
			return "", nil, err
		}
	default:
		return UnknownRequestType(event)
	}
	return event.LogicalResourceID, nil, nil
}

func deleteDatabase(conn *pgx.Conn, properties Properties) error {
	stmts := []string{
		fmt.Sprintf("drop database %s", properties.DatabaseName),
		fmt.Sprintf("drop user %s", properties.DatabaseName),
	}
	return executeStatements(conn, stmts)
}

func createDatabase(conn *pgx.Conn, properties Properties) error {
	stmts := []string{
		fmt.Sprintf("create database %s", properties.DatabaseName),
		fmt.Sprintf("create user %s with login", properties.DatabaseName),
		fmt.Sprintf("grant rds_iam to %s", properties.DatabaseName),
		fmt.Sprintf("grant all privileges on database %[1]s to %[1]s", properties.DatabaseName),
	}
	return executeStatements(conn, stmts)
}

func updateDatabase(conn *pgx.Conn, oldProperties, properties Properties) error {
	stmts := []string{
		fmt.Sprintf("alter database %s rename to %s", oldProperties.DatabaseName, properties.DatabaseName),
		fmt.Sprintf("alter user %s rename to %s", oldProperties.DatabaseName, properties.DatabaseName),
	}
	return executeStatements(conn, stmts)
}

func executeStatements(conn *pgx.Conn, statements []string) error {
	for _, stmt := range statements {
		tag, err := conn.Exec(stmt)
		if err != nil {
			return errors.Wrapf(err, "could not execute the statement %s, receives %s", stmt, tag)
		}
	}
	return nil
}

type DbInstanceInfo struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port     uint16 `json:"port"`
}

func (p *proc) dbInstanceInfo(ctx context.Context, properties Properties) (DbInstanceInfo, error) {
	var dbInstanceInfo DbInstanceInfo
	secret, err := p.smg.GetSecretValueRequest(&secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	}).Send(ctx)
	if err != nil {
		return DbInstanceInfo{}, errors.Wrapf(err, "could not fetch the secret %s", secretName)
	}
	var secretBytes []byte
	if secret.SecretString != nil {
		secretBytes = []byte(*secret.SecretString)
	} else {
		secretBytes = make([]byte, base64.StdEncoding.DecodedLen(len(secret.SecretBinary)))
		_, err := base64.StdEncoding.Decode(secretBytes, secret.SecretBinary)
		if err != nil {
			return DbInstanceInfo{}, errors.Wrapf(err, "cannot base64 decode the secret %s", secretName)
		}
	}
	var secretContent map[string]interface{}
	if err := json.Unmarshal(secretBytes, &secretContent); err != nil {
		return DbInstanceInfo{}, errors.Wrapf(err, "cannot json unmarshal the secret %s", secretName)
	}
	if err := mapstructure.Decode(secretContent, &dbInstanceInfo); err != nil {
		return dbInstanceInfo, err
	}
	return dbInstanceInfo, nil
}

func (p *proc) connect(ctx context.Context, properties Properties, database string) (*pgx.Conn, error) {
	dbInstanceInfo, err := p.dbInstanceInfo(ctx, properties)
	if err != nil {
		return nil, err
	}
	log.Debugw("connection config",
		"host", dbInstanceInfo.Host,
		"port", dbInstanceInfo.Port,
		"user", dbInstanceInfo.Username)
	tlsConfig, err := loadTlsConfig(dbInstanceInfo.Host)
	if err != nil {
		return nil, err
	}
	log.Debugw("tls config",
		"root cert CA count", len(tlsConfig.RootCAs.Subjects()))
	config := pgx.ConnConfig{
		Host:      dbInstanceInfo.Host,
		Port:      dbInstanceInfo.Port,
		User:      dbInstanceInfo.Username,
		Database:  database,
		Password:  dbInstanceInfo.Password,
		TLSConfig: tlsConfig,
	}
	conn, err := pgx.Connect(config)
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to the database %s", dbInstanceIdentifier)
	}
	return conn, nil
}

const rootCertificateFile = "rds-ca-2015-root.pem"

func loadTlsConfig(host string) (*tls.Config, error) {
	certPool := x509.NewCertPool()
	lambdaRootPath := os.Getenv("LAMBDA_TASK_ROOT")
	var certificateFile string
	if lambdaRootPath == "" {
		certificateFile = rootCertificateFile
	} else {
		certificateFile = path.Join(lambdaRootPath, rootCertificateFile)
	}
	pemCert, err := ioutil.ReadFile(certificateFile)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read the root certificate file %s", rootCertificateFile)
	}
	certPool.AppendCertsFromPEM(pemCert)
	return &tls.Config{
		RootCAs:    certPool,
		ServerName: host,
	}, nil
}

// ## Logging functions based on zap

func MustSugaredLogger() *zap.SugaredLogger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger.Sugar()
}

func SyncSugaredLogger(logger *zap.SugaredLogger) {
	if err := logger.Sync(); err != nil {
		fmt.Printf("could not sync sugared logger: %v", err)
	}
}

// ## Standard AWS config

func MustConfig(configs ...external.Config) aws.Config {
	cfg, err := external.LoadDefaultAWSConfig(configs...)
	if err != nil {
		panic(err)
	}
	return cfg
}

// ## Request Type

func UnknownRequestType(event cfn.Event) (string, map[string]interface{}, error) {
	return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
}
