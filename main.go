package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	dirRoot    = "/etc/awskmsluks/"
	configPath = "config.json"
	keyStore   = "keys/"
)

func main() {
	format := flag.Bool("format", false, "Generate passphrase for cryptsetup luksFormat")
	uuid := flag.String("uuid", "", "Return the passphrase to open the LUKS device. Value must be the UUID of the device")
	flag.Parse()
	if !*format && *uuid == "" {
		panic("either -format or -open must be passed")
	}

	if *format {
		err := formatDev()
		if err != nil {
			panic(err.Error())
		}
	} else {
		err := openDev(*uuid)
		if err != nil {
			panic(err.Error())
		}
	}
}

func formatDev() error {
	// Get host's FQDN
	fqdn, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("could not get host's FQDN: " + err.Error())
	}

	// Load configuration
	c, err := loadConfig(dirRoot + configPath)
	if err != nil {
		return err
	}

	// Generate a new data key
	key, err := newDataKey(c.CMKARN, fqdn, c.Production)
	if err != nil {
		return fmt.Errorf("could not generate the data key: " + err.Error())
	}

	// Archive the key to S3
	err = key.archive(c.KeyArchiveBucket)
	if err != nil {
		return fmt.Errorf("could not archive the data key: " + err.Error())
	}

	// Store the key locally
	err = key.store(dirRoot + keyStore)
	if err != nil {
		return err
	}

	// Success print outputs
	fmt.Fprintf(os.Stderr, "IMPORTANT: Update volume's UUID after the format operation has completed with:\ncryptsetup luksUUID <device> --uuid %s\n", key.EncryptionContext.UUID)
	fmt.Print(key.DataKey.Plain)
	return nil
}

func openDev(uuid string) error {
	// Get host's FQDN
	fqdn, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("could not get host's FQDN: " + err.Error())
	}

	path := dirRoot + keyStore + fqdn + "/" + uuid + ".json"
	var k key
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading device's key from local store (%s): %v", path, err)
	}
	err = json.Unmarshal(b, &k)
	if err != nil {
		return fmt.Errorf("error parsing device's key from local store (%s): %v", path, err)
	}
	err = k.decrypt()
	if err != nil {
		return fmt.Errorf("could not decrypt key: %v", err)
	}
	fmt.Print(k.DataKey.Plain)
	return nil
}
