package main

import (
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
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

// Process input values and generate vesting and vault accounts.
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
		VestingStart: types.LayerID(constants.VestStart),
		VestingEnd:   types.LayerID(constants.VestEnd),
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

	// Audit total vaulted supply
	vaultTotal := uint64(0)

	// Disable go-spacemesh logging, it gets printed to STDOUT and messes up the CSV
	smlog.SetupGlobal(smlog.NewNop())

	// Set the HRP, we only have to do this once
	types.SetNetworkHRP(hrp)

	cw := csv.NewWriter(os.Stdout)
	// Write headers to the output CSV
	cw.Write([]string{"Name", "AmountInitialSmidge", "AmountTotalSmidge", "TemplateAddress", "VestingAddress", "VaultAddress", "VestStart", "VestEnd"})

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
		// Convert amount units from SMH -> smidge
		if amount > math.MaxUint64/constants.OneSmesh {
			log.Fatal("math overflow")
		}
		amountSmidge := amount * constants.OneSmesh

		if err != nil {
			log.Printf("Warning: Invalid amount for record at line %d: %s\n", line, name)
			continue
		}

		var keys []core.PublicKey
		for _, keyStr := range record[2:7] {
			if keyStr == "" {
				continue
			}

			keyBytes, err := hex.DecodeString(keyStr)
			if err != nil || len(keyBytes) != ed25519.PublicKeySize {
				log.Printf("Error: Invalid key for record at line %d: %s\n", line, name)
				continue
			}
			key := [ed25519.PublicKeySize]byte{}
			copy(key[:], keyBytes)
			keys = append(keys, key)
		}
		if len(keys) == 0 {
			log.Printf("Error: No keys for record at line %d: %s\n", line, name)
			continue
		}

		mStr, nStr := record[7], record[8]
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
		if m != uint8(len(keys)) {
			log.Printf("Error: m doesn't match number of keys at line %d: %s\n", line, name)
			continue
		}

		vaultTotal += amount
		templateAddress, vestingAddress, vaultAddress, amountInitial, vestStart, vestEnd := processKeys(keys, m, amountSmidge)

		// Use csv.Writer to correctly escape names that contain commas
		cw.Write([]string{
			//name,
			fmt.Sprintf("record%d", line),
			strconv.FormatUint(amountInitial, 10),
			strconv.FormatUint(amountSmidge, 10),
			templateAddress,
			vestingAddress,
			vaultAddress,
			strconv.FormatUint(uint64(vestStart), 10),
			strconv.FormatUint(uint64(vestEnd), 10),
		})
		cw.Flush()
	}
	log.Printf("Total vaulted issuance: %d SMH", vaultTotal)
	if vaultTotal*constants.OneSmesh != constants.TotalVaulted {
		log.Printf("ERROR: expected total issuance of %d SMH", constants.TotalVaulted/constants.OneSmesh)
	}
}
