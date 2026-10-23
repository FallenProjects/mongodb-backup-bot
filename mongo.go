package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

func parseMongoRequest(text string) (uri, format string, importing bool, err error) {
	format = "gz"
	fields := strings.Fields(text)
	for _, field := range fields {
		if strings.HasPrefix(field, "mongodb://") || strings.HasPrefix(field, "mongodb+srv://") {
			if uri != "" {
				return "", "", false, errors.New("provide one MongoDB URI per command")
			}
			uri = field
		}
	}

	if uri == "" {
		return "", "", false, errors.New("use /mongo <URI> with optional {gz}, {zip}, or {import}")
	}

	parsed, parseErr := connstring.Parse(uri)
	if parseErr != nil {
		return "", "", false, errors.New("invalid MongoDB URI; percent-encode special characters in credentials")
	}

	gzip := false
	for _, field := range fields {
		switch strings.ToLower(field) {
		case "{gz}":
			gzip = true
		case "{json}", "{zip}":
			format = "zip"
		case "{import}":
			importing = true
		}
	}

	if gzip {
		format = "gz"
	}

	if importing && parsed.Database != "" {
		return "", "", false, errors.New("use a restore URI without a database path; set authSource in the query if needed. Database names are preserved from the backup")
	}

	return uri, format, importing, nil
}

func systemDatabase(name string) bool {
	return name == "admin" || name == "config" || name == "local"
}

func connectMongo(uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(10 * time.Second).SetConnectTimeout(10 * time.Second))
	if err != nil {
		return nil, errors.New("could not connect to MongoDB; check the URI, network access and credentials")
	}

	return client, nil
}

func disconnectMongo(client *mongo.Client) {
	_ = client.Disconnect(context.Background())
}

func databaseNames(uri string) ([]string, error) {
	client, err := connectMongo(uri)
	if err != nil {
		return nil, err
	}
	defer disconnectMongo(client)
	parsed, err := connstring.Parse(uri)
	if err != nil {
		return nil, errors.New("invalid MongoDB URI")
	}

	if parsed.Database != "" {
		if systemDatabase(parsed.Database) {
			return nil, errors.New("system databases cannot be selected or wiped")
		}
		if err = client.Database(parsed.Database).RunCommand(context.Background(), bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
			return nil, errors.New("could not access the database; check credentials and network access")
		}
		return []string{parsed.Database}, nil
	}

	names, err := client.ListDatabaseNames(context.Background(), bson.D{})
	if err != nil {
		return nil, errors.New("could not list databases; check credentials, listDatabases permission and network access")
	}

	result := make([]string, 0, len(names))
	for _, name := range names {
		if !systemDatabase(name) {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result, nil
}

func wipeDatabases(uri string) error {
	names, err := databaseNames(uri)
	if err != nil {
		return err
	}
	client, err := connectMongo(uri)
	if err != nil {
		return err
	}
	defer disconnectMongo(client)
	for _, name := range names {
		if systemDatabase(name) {
			return errors.New("refusing to wipe a system database")
		}
		db := client.Database(name)
		if err := db.Drop(context.Background()); err == nil {
			continue
		} else {
			var serverErr mongo.ServerError
			if !errors.As(err, &serverErr) || (!serverErr.HasErrorCode(13) && !serverErr.HasErrorCode(8000)) {
				return errors.New("database deletion failed; some databases may already have been removed")
			}
		}

		collections, err := db.ListCollectionNames(context.Background(), bson.D{})
		if err != nil {
			return errors.New("could not list collections for deletion; the wipe may be partial")
		}
		for _, collection := range collections {
			if strings.HasPrefix(collection, "system.") {
				continue
			}
			if err := db.Collection(collection).Drop(context.Background()); err != nil {
				return errors.New("collection deletion failed; the wipe may be partial")
			}
		}
	}
	return nil
}

func runMongoTool(tool, uri, dir string, args ...string) error {
	encoded, err := json.Marshal(uri)
	if err != nil {
		return err
	}
	configPath := filepath.Join(dir, "mongo.yml")
	if err = os.WriteFile(configPath, append(append([]byte("uri: "), encoded...), '\n'), 0600); err != nil {
		return errors.New("could not write temporary MongoDB configuration")
	}
	defer os.Remove(configPath)
	cmd := exec.Command(tool, append([]string{"--config=" + configPath, "--quiet"}, args...)...)

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("%s failed; check database access, tool compatibility and the backup format", tool)
	}
	return nil
}

func dumpMongo(uri, format, database, dir string) (string, error) {
	var args []string
	if database != "" {
		args = append(args, "--db="+database)
	}

	name := "mongo_backup_" + time.Now().UTC().Format("20060102-150405")
	if format == "gz" {
		path := filepath.Join(dir, name+".gz")
		err := runMongoTool("mongodump", uri, dir, append(args, "--archive="+path, "--gzip")...)
		return path, err
	}

	if format != "zip" {
		return "", errors.New("unsupported backup format")
	}
	dumpDir := filepath.Join(dir, "dump")
	if err := runMongoTool("mongodump", uri, dir, append(args, "--out="+dumpDir)...); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".zip")
	return path, zipDirectory(dumpDir, path)
}

func restoreMongo(uri, path, ext, dir string) error {
	if ext == ".gz" {
		return runMongoTool("mongorestore", uri, dir, "--archive="+path, "--gzip", "--stopOnError")
	}
	if ext != ".zip" {
		return errors.New("only .gz archives and .zip BSON dumps can be restored")
	}
	dumpDir := filepath.Join(dir, "dump")
	if err := extractZip(path, dumpDir); err != nil {
		return err
	}
	return runMongoTool("mongorestore", uri, dir, "--dir="+dumpDir, "--stopOnError")
}
