package service

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func GenerateFileHash(filePath string) (string, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	defer file.Close()

	hash := sha256.New()

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	hashBytes := hash.Sum(nil)

	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil
}