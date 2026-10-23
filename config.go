package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	apiID    int32
	apiHash  string
	token    string
	ownerID  int64
	pageSize = 8
)

func loadConfig() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}
	id, err := strconv.ParseInt(os.Getenv("API_ID"), 10, 32)
	if err != nil || id <= 0 {
		return fmt.Errorf("API_ID must be a positive integer")
	}

	apiID, apiHash, token = int32(id), os.Getenv("API_HASH"), os.Getenv("TOKEN")
	if apiHash == "" || token == "" {
		return fmt.Errorf("API_HASH and TOKEN are required")
	}

	if value := os.Getenv("OWNER_ID"); value != "" {
		ownerID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || ownerID < 0 {
			return fmt.Errorf("OWNER_ID must be a nonnegative user ID")
		}
	}

	if value := os.Getenv("MONGODB_BACKUP_PAGE_SIZE"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 40 {
			return fmt.Errorf("MONGODB_BACKUP_PAGE_SIZE must be between 1 and 40")
		}
	}

	return nil
}
