package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/codesmith-gmbh/cgc/cgcaws"
	"github.com/codesmith-gmbh/cgc/cgccf"
	"github.com/codesmith-gmbh/cgc/cgclog"
	"github.com/codesmith-gmbh/cgc/cgcpg"
	"github.com/jackc/pgx"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"os"
)

const (
	postgrepDatabase = "postgres"
)

var (
	log = cgclog.MustSugaredLogger()
)

type proc struct {
	smg                           *secretsmanager.Client
	dbInstanceIdentifier, groupId string
}

func newProc(cfg aws.Config, dbInstanceIdentifier, groupId string) *proc {
	return &proc{
		smg:                  secretsmanager.New(cfg),
		dbInstanceIdentifier: dbInstanceIdentifier,
		groupId:              groupId,
	}
}

func main() {
	defer cgclog.SyncSugaredLogger(log)
	dbInstanceIdentifier := os.Getenv("DB_INSTANCE_IDENTIFIER")
	groupId := os.Getenv("DB_INSTANCE_SECURITY_GROUP")
	ingressDescription := os.Getenv("AWS_LAMBDA_LOG_STREAM_NAME")
	p, sgs := initLambda(dbInstanceIdentifier, groupId, ingressDescription)
	if sgs != nil {
		//noinspection GoUnhandledErrorResult
		defer sgs.EnsureDescribedIngressRevoked(context.TODO(), groupId, ingressDescription)
	}
	cgccf.StartEventProcessor(p)
}

func initLambda(dbInstanceIdentifier, groupId, ingressDescription string) (cgccf.EventProcessor, *cgcaws.SGS) {
	if dbInstanceIdentifier == "" {
		return constantErrorProcessorWithMsg("dbInstanceIdentifier not defined")
	}
	if groupId == "" {
		return constantErrorProcessorWithMsg("groupId not defined")
	}
	if ingressDescription == "" {
		return constantErrorProcessorWithMsg("ingressDescription not defined")
	}
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return &cgccf.ConstantErrorEventProcessor{Error: err}, nil
	}
	ec2Client := ec2.New(cfg)
	sgs := cgcaws.NewAwsSecurityGroupService(ec2Client)
	err = sgs.OpenSecurityGroup(context.TODO(), groupId, ingressDescription)
	if err != nil {
		return &cgccf.ConstantErrorEventProcessor{Error: err}, sgs
	}
	return newProc(cfg, dbInstanceIdentifier, groupId), sgs
}

func constantErrorProcessorWithMsg(message string) (cgccf.EventProcessor, *cgcaws.SGS) {
	return &cgccf.ConstantErrorEventProcessor{Error: errors.New(message)}, nil
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

func (p *proc) ProcessEvent(ctx context.Context, event cfn.Event) (string, map[string]interface{}, error) {
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
	secretName := p.secretName()
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

func (p *proc) secretName() string {
	return "codesmith-forge--rds--" + p.dbInstanceIdentifier
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
	config := pgx.ConnConfig{
		Host:     dbInstanceInfo.Host,
		Port:     dbInstanceInfo.Port,
		User:     dbInstanceInfo.Username,
		Database: database,
		Password: dbInstanceInfo.Password,
	}
	tlsConfigOption := &cgcpg.TlsConfigOption{Host: dbInstanceInfo.Host}
	if err := tlsConfigOption.Configure(&config); err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(config)
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to the database %s", p.dbInstanceIdentifier)
	}
	return conn, nil
}

// ## Request Type

func UnknownRequestType(event cfn.Event) (string, map[string]interface{}, error) {
	return event.PhysicalResourceID, nil, errors.Errorf("unknown request type %s", event.RequestType)
}
