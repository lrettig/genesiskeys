package main

import (
	"encoding/csv"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ed25519"
)

// Placeholder function: replace this with your function
func processKeys(keys []ed25519.PublicKey, m, n uint) string {
	return "Result" // replace with your processing
}

func main() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatal("Please provide one CSV file as an argument")
	}

	filePath := flag.Args()[0]
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Could not open the CSV file: ", err)
	}

	r := csv.NewReader(file)

	// Skip the first line (header)
	_, err = r.Read()
	if err != nil {
		log.Fatal("Could not read the CSV file: ", err)
	}

	line := 1

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("Could not read the CSV file: ", err)
		}

		line++

		name := record[0]

		amountStr := strings.ReplaceAll(record[1], ",", "")
		amount, err := strconv.ParseUint(amountStr, 10, 64)
		if err != nil {
			log.Printf("Warning: Invalid amount for record at line %d: %s\n", line, name)
			continue
		}

		var keys []ed25519.PublicKey
		for _, keyStr := range record[4:8] {
			if keyStr == "" {
				continue
			}

			keyBytes, err := hex.DecodeString(keyStr)
			if err != nil || len(keyBytes) != ed25519.PublicKeySize {
				log.Printf("Error: Invalid key for record at line %d: %s\n", line, name)
				continue
			}

			keys = append(keys, ed25519.PublicKey(keyBytes))
		}

		mStr, nStr := record[6], record[7]
		var m, n uint
		if mStr != "" || nStr != "" {
			if mStr == "" || nStr == "" {
				log.Printf("Error: Only one of m or n is specified for record at line %d: %s\n", line, name)
				continue
			}

			mUint64, err := strconv.ParseUint(mStr, 10, 64)
			if err != nil {
				log.Printf("Error: Invalid m for record at line %d: %s\n", line, name)
				continue
			}
			m = uint(mUint64)

			nUint64, err := strconv.ParseUint(nStr, 10, 64)
			if err != nil {
				log.Printf("Error: Invalid n for record at line %d: %s\n", line, name)
				continue
			}
			n = uint(nUint64)

			if int(n) != len(keys) {
				log.Printf("Error: n does not match the number of keys for record at line %d: %s\n", line, name)
				continue
			}
		}

		result := processKeys(keys, m, n)
		// Use csv.Writer to correctly escape names that contain commas
		cw := csv.NewWriter(os.Stdout)
		cw.Write([]string{name, strconv.FormatUint(amount, 10), result})
		cw.Flush()
	}
}
