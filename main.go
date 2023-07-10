package main

import (
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ed25519"

	"github.com/spacemeshos/economics/constants"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/genvm/core"
	"github.com/spacemeshos/go-spacemesh/genvm/templates/multisig"
	"github.com/spacemeshos/go-spacemesh/genvm/templates/vault"
	"github.com/spacemeshos/go-spacemesh/genvm/templates/vesting"
	smlog "github.com/spacemeshos/go-spacemesh/log"
)

const hrp = "sm"

// Placeholder function: replace this with your function
func processKeys(keys []core.PublicKey, numRequired uint8, amountTotal uint64) (templateAddress, vestingAddress, vaultAddress string, amountInitial uint64, vestStart, vestEnd uint32) {
	vestingArgs := &multisig.SpawnArguments{
		Required:   numRequired,
		PublicKeys: keys,
	}
	vestingAccount := core.ComputePrincipal(vesting.TemplateAddress, vestingArgs)

	// initial amount is 25% of final amount
	amountInitial = amountTotal / 4

	vaultArgs := &vault.SpawnArguments{
		Owner:               vestingAccount,
		TotalAmount:         amountTotal,
		InitialUnlockAmount: amountInitial,
		// All genesis vaults have the same vesting schedule
		VestingStart: types.LayerID(uint32(constants.VestStart)),
		VestingEnd:   types.LayerID(uint32(constants.VestEnd)),
	}
	vaultAccount := core.ComputePrincipal(vault.TemplateAddress, vaultArgs)
	log.Printf("vesting: %s\nvault: %s\n", vestingAccount.String(), vaultAccount.String())
	log.Println("public keys:")
	for i, key := range vestingArgs.PublicKeys {
		log.Printf("%d: 0x%x\n", i, key[:])
	}
	return vesting.TemplateAddress.String(), vestingAccount.String(), vaultAccount.String(), amountInitial, uint32(constants.VestStart), uint32(constants.VestEnd)
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

	// Disable go-spacemesh logging, it gets printed to STDOUT and messes up the CSV
	smlog.SetupGlobal(smlog.NewNop())

	// Set the HRP, we only have to do this once
	types.SetNetworkHRP(hrp)

	cw := csv.NewWriter(os.Stdout)
	// Write headers to the output CSV
	cw.Write([]string{"Name", "AmountInitial", "AmountTotal", "TemplateAddress", "VestingAddress", "VaultAddress", "VestStart", "VestEnd"})

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

		var keys []core.PublicKey
		for _, keyStr := range record[4:8] {
			if keyStr == "" {
				continue
			}

			keyBytes, err := hex.DecodeString(keyStr)
			key := [ed25519.PublicKeySize]byte{}
			if err != nil || len(keyBytes) != ed25519.PublicKeySize {
				log.Printf("Error: Invalid key for record at line %d: %s\n", line, name)
				continue
			}
			copy(key[:], keyBytes)
			keys = append(keys, key)
		}

		mStr, nStr := record[8], record[9]
		var m, n uint8
		if mStr != "" || nStr != "" {
			if mStr == "" || nStr == "" {
				log.Printf("Error: Only one of m or n is specified for record at line %d: %s\n", line, name)
				continue
			}

			mUint64, err := strconv.ParseUint(mStr, 10, 8)
			if err != nil {
				log.Printf("Error: Invalid m for record at line %d: %s\n", line, name)
				continue
			}
			m = uint8(mUint64)

			nUint64, err := strconv.ParseUint(nStr, 10, 8)
			if err != nil {
				log.Printf("Error: Invalid n for record at line %d: %s\n", line, name)
				continue
			}
			n = uint8(nUint64)

			if int(n) != len(keys) {
				log.Printf("Error: n does not match the number of keys for record at line %d: %s\n", line, name)
				continue
			}
		}

		templateAddress, vestingAddress, vaultAddress, amountInitial, vestStart, vestEnd := processKeys(keys, m, amount)
		// Use csv.Writer to correctly escape names that contain commas
		cw.Write([]string{
			//name,
			fmt.Sprintf("record%d", line),
			strconv.FormatUint(amountInitial, 10),
			strconv.FormatUint(amount, 10),
			templateAddress,
			vestingAddress,
			vaultAddress,
			strconv.FormatUint(uint64(vestStart), 10),
			strconv.FormatUint(uint64(vestEnd), 10),
		})
		cw.Flush()
	}
}
